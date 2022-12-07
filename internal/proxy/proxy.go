// Package proxy adds abstraction to make working with http proxies easier
package proxy

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// Proxy is a url.URL specialized for proxies
type Proxy struct {
	*url.URL
}

var (
	errInvalidProxy = errors.New("invalid proxy")
)

// Validate checks if a proxy is valid
func (p *Proxy) Validate() error {
	if p == nil {
		return errors.New("nil proxy")
	}
	if p.Scheme != "http" ||
		p.Opaque != "" ||
		p.Path != "" ||
		p.RawQuery != "" ||
		p.Fragment != "" ||
		p.Host == "" {
		return errInvalidProxy
	}
	return nil
}

// String overloads url.URL's String interface method to return an empty string on invalid proxies,
func (p *Proxy) String() string {
	if err := p.Validate(); err != nil {
		return ""
	}
	return p.URL.String()
}

// FromString converts a proxy in string format (user:pass:host:port, user:pass@host:port or host:port to a Proxy structure
func FromString(proxy string) (*Proxy, error) {
	var (
		proxyUrl           string
		userInfo, hostInfo []string
	)

	if strings.Contains(proxy, "@") { // user:pass@host:port
		if strings.Count(proxy, "@") > 1 {
			return nil, errInvalidProxy
		}

		parts := strings.Split(proxy, "@")
		userInfo = strings.Split(parts[0], ":")
		hostInfo = strings.Split(parts[1], ":")
		if len(userInfo) != 2 || len(hostInfo) != 2 {
			return nil, errInvalidProxy
		}
	} else { // user:pass:host:port, user:pass@host:port or host:port
		parts := strings.Split(proxy, ":")

		switch len(parts) {
		case 4:
			userInfo = parts[:2]
			hostInfo = parts[2:]
		case 2:
			hostInfo = parts
		default:
			return nil, errInvalidProxy
		}
	}

	if hostInfo[0] == "" {
		return nil, errInvalidProxy
	}

	if len(userInfo) > 0 {
		proxyUrl = "http://" + userInfo[0] + ":" + userInfo[1] + "@" + hostInfo[0] + ":" + hostInfo[1]
	} else {
		proxyUrl = "http://" + hostInfo[0] + ":" + hostInfo[1]
	}

	parsed, err := url.Parse(proxyUrl)
	if err != nil {
		return nil, err
	}

	proxyObj := &Proxy{parsed}

	if err = proxyObj.Validate(); err != nil {
		return nil, err
	}

	return proxyObj, nil
}

// FromReader takes in a reader and returns a slice of *Proxy.
// Proxies should be separated by new lines.
// Returns an error if any proxy is invalid
func FromReader(r io.Reader) ([]*Proxy, error) {
	var (
		proxies []*Proxy
		idx     int
	)
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		proxy, err := FromString(sc.Text())
		if err != nil {
			return nil, fmt.Errorf("invalid proxy: %v (%w)", idx, err)
		}
		idx += 1
		proxies = append(proxies, proxy)
	}
	return proxies, nil
}
