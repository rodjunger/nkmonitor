// Package monitor implements a monitor for nike.com.br product restocks
package nkmonitor

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
	"github.com/rodjunger/nkmonitor/internal/proxy"
	http "github.com/saucesteals/fhttp"
	"github.com/saucesteals/fhttp/cookiejar"
	"github.com/saucesteals/mimic"
	"github.com/tidwall/gjson"
	"go.uber.org/atomic"
)

type Monitor struct {
	started               *atomic.Bool
	defaultClient         *http.Client
	userAgent             string
	mimicSpec             *mimic.ClientSpec
	addTaskCh             chan monitorTask
	removeTaskCh          chan string
	stopCh                chan chan struct{}
	buildID               *atomic.String
	lastBuildIdUpdateTime time.Time
	buildIdUpdateLock     *sync.Mutex
	startStopLock         *sync.Mutex
	delay                 time.Duration
	proxies               []*proxy.Proxy
	curProxyIndex         *atomic.Uint64
	//logger              log.Logger
}

// SizeInfo stores detailed information of a specific product SKU
type SizeInfo struct {
	Description string // Description is the name of the specific size, example: 43, UNICO, 35,5
	Sku         string // Sku in the product SKU in their internal SKU model, used for adding to cart
	Ean         string // Ean is the global identifier of the product, also available in the product label
	HasStock    bool   // HasStock defines whether a size has stock or not
	IsAvailable bool   // Only Available products can be added to cart
	Restocked   bool   // Restocked specifies whether this specific size just restocked
}

type RestockInfo struct {
	Path     string // Url of the restocked product
	Name     string
	NickName string
	Code     string
	Price    string
	Picture  string
	Sizes    []*SizeInfo // List of products that have stock or are available (not just the ones that just restocked)
}

type monitorTask struct {
	path     string
	callback chan RestockInfo
	id       string
}

var (
	defaultMasterHeaderOrder = []string{
		"sec-ch-ua",
		"sec-ch-ua-mobile",
		"sec-ch-ua-platform",
		"dnt",
		"upgrade-insecure-requests",
		"user-agent",
		"accept",
		"sec-fetch-site",
		"sec-fetch-mode",
		"sec-fetch-user",
		"sec-fetch-dest",
		"referer",
		"accept-encoding",
		"accept-language",
		"cookie",
	}
	errBuildIDAlreadyUpdated = errors.New("buildID is already updated")
	errInvalidJson           = errors.New("invalid JSON")
	ErrAlreadyStarted        = errors.New("monitor already started")
	ErrNotStarted            = errors.New("monitor not started")
	ErrInvalidUserAgent      = errors.New("invalid user-agent")
	errNoProxiesAvailable    = errors.New("no proxies available")
	ErrInvalidUrl            = errors.New("invalid URL")
	ErrNilCallback           = errors.New("nil callback")
)

func (m *Monitor) getProxy() (string, error) {
	if len(m.proxies) > 0 {
		return m.proxies[m.curProxyIndex.Inc()%uint64(len(m.proxies))].String(), nil
	}
	return "", errNoProxiesAvailable
}

// NewMonitor is used to create and initialize a new Monitor struct with sane defaults and error checking.
// It takes as input a userAgent string representing the user agent to be used when making requests to the website,
// a delay duration representing the amount of time to wait between requests,
// a slice of proxies containing the proxy URLs to be used for requests, and a *mimic.ClientSpec to configure the http clients.
func NewMonitor(userAgent string, delay time.Duration, proxies []string, mimicSpec *mimic.ClientSpec) (*Monitor, error) {
	if userAgent == "" {
		return nil, ErrInvalidUserAgent
	}

	if mimicSpec == nil {
		return nil, errors.New("mimic spec cannot be nil")
	}

	if delay < time.Second {
		return nil, errors.New("delay too low")
	}

	var parsedProxies []*proxy.Proxy

	for _, rawProxy := range proxies {
		if parsed, err := proxy.FromString(rawProxy); err == nil {
			parsedProxies = append(parsedProxies, parsed)
		} else {
			return nil, err
		}
	}

	monitor := Monitor{
		started:               &atomic.Bool{},
		defaultClient:         nil,
		userAgent:             userAgent,
		mimicSpec:             mimicSpec,
		addTaskCh:             make(chan monitorTask, 1),
		removeTaskCh:          make(chan string),
		stopCh:                make(chan chan struct{}),
		buildID:               &atomic.String{},
		lastBuildIdUpdateTime: time.Now().Add(-999 * time.Hour),
		buildIdUpdateLock:     &sync.Mutex{},
		startStopLock:         &sync.Mutex{},
		delay:                 delay,
		proxies:               parsedProxies,
		curProxyIndex:         &atomic.Uint64{},
	}

	monitor.defaultClient = monitor.newHttpClient()

	return &monitor, nil
}

func (m *Monitor) newHttpClient() *http.Client {
	//This function never returns an err != nil (checked on source code)
	jar, _ := cookiejar.New(nil)
	var newClient *http.Client
	if proxy, err := m.getProxy(); err == nil {
		proxyUrl, _ := url.Parse(proxy)
		newClient = &http.Client{Jar: jar, Transport: m.mimicSpec.ConfigureTransport(&http.Transport{Proxy: http.ProxyURL(proxyUrl)}), Timeout: 20 * time.Second}
	} else {
		newClient = &http.Client{Jar: jar, Transport: m.mimicSpec.ConfigureTransport(&http.Transport{}), Timeout: 20 * time.Second}
	}
	return newClient
}

func (m *Monitor) generateMonitorUrl(path string) string {
	return "https://www.nike.com.br/_next/data/" + m.buildID.Load() + path + ".json"
}

func (m *Monitor) performGet(client *http.Client, url string) (body []byte, statusCode int, err error) {
	headerOrder := make([]string, len(defaultMasterHeaderOrder))
	copy(headerOrder, defaultMasterHeaderOrder)

	headers := http.Header{
		"sec-ch-ua":                 {m.mimicSpec.ClientHintUA()},
		"sec-ch-ua-mobile":          {"?0"},
		"sec-ch-ua-platform":        {"\"Windows\""},
		"dnt":                       {"1"},
		"upgrade-insecure-requests": {"1"},
		"user-agent":                {m.userAgent},
		"accept":                    {"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9"},
		"sec-fetch-site":            {"none"},
		"sec-fetch-mode":            {"navigate"},
		"sec-fetch-user":            {"?1"},
		"sec-fetch-dest":            {"document"},
		"accept-encoding":           {"gzip, deflate, br"},
		"accept-language":           {"pt,pt-PT;q=0.9,en-US;q=0.8,en;q=0.7,es;q=0.6"},
		http.PHeaderOrderKey:        m.mimicSpec.PseudoHeaderOrder(),
		http.HeaderOrderKey:         headerOrder,
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)

	if err != nil {
		return nil, 0, err
	}

	req.Header = headers

	resp, err := client.Do(req)

	if err != nil {
		return nil, 0, err
	}

	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)

	if err != nil {
		return nil, 0, err
	}

	return body, resp.StatusCode, nil

}

func (m *Monitor) monitorProduct(cancel <-chan struct{}, productPath string, notify chan<- RestockInfo) {
	var (
		backendUrl           = m.generateMonitorUrl(productPath)
		localClient          = m.newHttpClient()
		previouslyAvailable  = map[string]bool{}
		previouslyInStock    = map[string]bool{}
		lastRequestStartTime = time.Now().Add(-m.delay)
	)

	for {
		select {
		case <-cancel:
			return
		default:
		}

		<-time.After(time.Until(lastRequestStartTime.Add(m.delay)))

		lastRequestStartTime = time.Now()

		body, statusCode, err := m.performGet(localClient, backendUrl)

		if err != nil {
			continue
		}

		var (
			products   []*SizeInfo
			hadRestock = false
			jsonString = string(body)
		)

		switch statusCode {
		case http.StatusOK:
			if !gjson.Valid(jsonString) {
				continue
			}
			product := gjson.Get(jsonString, "pageProps.product")
			sizes := product.Get("sizes").Array()
			for _, size := range sizes {
				thisSize := &SizeInfo{
					Description: size.Get("description").String(),
					Sku:         size.Get("sku").String(),
					Ean:         size.Get("ean").String(),
					HasStock:    size.Get("hasStock").Bool(),
					IsAvailable: size.Get("isAvailable").Bool(),
				}
				// Checks if it was previously not in stock but is now, or if it was not available but is now. In stock means what it says, but it can only be added to cart when it is Available
				if thisSize.IsAvailable && !previouslyAvailable[thisSize.Sku] || thisSize.HasStock && !previouslyInStock[thisSize.Sku] {
					thisSize.Restocked = true
					hadRestock = true
				}

				if thisSize.IsAvailable || thisSize.HasStock {
					products = append(products, thisSize)
				}

				previouslyAvailable[thisSize.Sku] = thisSize.IsAvailable
				previouslyInStock[thisSize.Sku] = thisSize.HasStock
			}

			if hadRestock {
				notify <- RestockInfo{
					Path:     productPath,
					Name:     product.Get("name").String(),
					NickName: product.Get("nickname").String(),
					Code:     product.Get("colorInfo.styleCode").String(),
					Price:    product.Get("priceInfos.priceFormatted").String(),
					Picture:  product.Get("images.0.url").String(),
					Sizes:    products,
				}
			}
		case http.StatusForbidden:
			localClient = m.newHttpClient()
		case http.StatusNotFound:
			m.updateBuildID()
			backendUrl = m.generateMonitorUrl(productPath)
		default:
		}
	}
}

func ParseNKUrl(productUrl string) (*url.URL, error) {
	parsed, err := url.Parse(productUrl)
	if err != nil {
		return nil, err
	}
	if productUrl == "" || parsed.Path == "" || (parsed.Host != "nike.com.br" && parsed.Host != "www.nike.com.br") {
		return nil, ErrInvalidUrl
	}

	return parsed, nil
}

// AddTask creates a new monitoring task for the desired url and callback channel, returns the uuid of the task
// so it can be stopped later with RemoveTask
func (m *Monitor) AddTask(productUrl string, callback chan RestockInfo) (string, error) {
	if m.started.Load() {
		//Shouldn't be a problem, but also there's no reason to do it so better to return an error
		if callback == nil {
			return "", ErrNilCallback
		}
		parsed, err := ParseNKUrl(productUrl)
		if err != nil {
			return "", err
		}
		newTask := monitorTask{path: parsed.Path, callback: callback, id: uuid.NewString()}
		m.addTaskCh <- newTask
		return newTask.id, nil
	} else {
		return "", ErrNotStarted
	}
}

// RemoveTask removes a task from the task list, it's a no-op if the monitor is stopped or the task does not exist
func (m *Monitor) RemoveTask(taskId string) {
	if !m.started.Load() {
		return
	}
	m.removeTaskCh <- taskId
}

func (m *Monitor) mainLoop() {
	var (
		updateNotifyCh = make(chan RestockInfo, 1)
		taskList       = make(map[string]map[string]monitorTask)
		cancelChs      = make(map[string]chan struct{})
	)

	for {
		select {
		case newTask := <-m.addTaskCh:
			if _, ok := taskList[newTask.path]; !ok {
				cancelChannel := make(chan struct{}, 1)
				go m.monitorProduct(cancelChannel, newTask.path, updateNotifyCh)
				taskList[newTask.path] = map[string]monitorTask{}
				cancelChs[newTask.path] = cancelChannel
			}
			taskList[newTask.path][newTask.id] = newTask
		case restockInfo := <-updateNotifyCh:
			for _, task := range taskList[restockInfo.Path] {
				//copy to avoid race conditions
				callback := task.callback
				info := restockInfo
				go func() {
					//Use a timeout to avoid a coroutine leak if the value is never received by the receiver
					select {
					case callback <- info:
					case <-time.After(60 * time.Second):
					}
				}()
			}
		case toRemove := <-m.removeTaskCh:
			// delete is a no-op is the value doesn't exist, this shouldn't be a performance hurdle and simplifies the code a little bit
			for key, list := range taskList {
				oldSize := len(list)
				delete(list, toRemove)
				newSize := len(list)
				if newSize == 0 && oldSize > 0 { // We just emptyed the map
					// Send cancellation to channel, it's buffered so no chance of locking
					cancelChs[key] <- struct{}{}
					// This is safe https://stackoverflow.com/questions/23229975/is-it-safe-to-remove-selected-keys-from-map-within-a-range-loop
					delete(taskList, key)
					delete(cancelChs, key)
				}
			}
		case pingBackCh := <-m.stopCh: // Stop all running monitors, send a ping back and stop the main loop
			for _, ch := range cancelChs {
				ch <- struct{}{}
			}
			pingBackCh <- struct{}{}
			return
		}
	}
}

// Start starts the monitors, needs to be called before calling AddTask
func (m *Monitor) Start() error {
	// Make sure we don't start twice
	m.startStopLock.Lock()
	defer m.startStopLock.Unlock()

	if m.started.Load() {
		return ErrAlreadyStarted
	}

	err := m.updateBuildID()

	if err != nil {
		return err
	}

	go m.mainLoop()
	m.started.Store(true)
	return nil
}

// Stop stops the monitor
func (m *Monitor) Stop() error {
	m.startStopLock.Lock()
	defer m.startStopLock.Unlock()

	if !m.started.Load() {
		return ErrNotStarted
	}

	defer m.started.Store(false)
	done := make(chan struct{})
	m.stopCh <- done
	<-done
	return nil
}

func (m *Monitor) updateBuildID() error {
	//Used to ensure that only one instance of this code runs at a certain time
	m.buildIdUpdateLock.Lock()
	defer m.buildIdUpdateLock.Unlock()

	//Verifies if it has been at least a minute since the buildId was succesfully updated
	if time.Since(m.lastBuildIdUpdateTime) < time.Minute {
		return errBuildIDAlreadyUpdated
	}

	body, statusCode, err := m.performGet(m.defaultClient, "https://www.nike.com.br/")

	if err != nil {
		return err
	}

	switch statusCode {
	case http.StatusOK:
		//probably no need to convert to string, needs testing
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))

		if err != nil {
			return err
		}

		jsonData := doc.Find("#__NEXT_DATA__").First().Text()

		if !gjson.Valid(jsonData) {
			return errInvalidJson
		}
		//maybe check if buildId is there, but should always be
		buildId := gjson.Get(jsonData, "buildId").String()
		m.buildID.Store(buildId)
		m.lastBuildIdUpdateTime = time.Now()
		return nil

	case http.StatusForbidden:
		m.defaultClient = m.newHttpClient()
		fallthrough

	default:
		return fmt.Errorf("updateBuildID failed: HTTP status != 200 (%v) getting main page", statusCode)
	}

}
