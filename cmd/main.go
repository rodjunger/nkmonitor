package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mileusna/useragent"
	"github.com/rodjunger/nkmonitor"
	"github.com/rodjunger/nkmonitor/cmd/notify"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/saucesteals/mimic"
	"github.com/spf13/cobra"
)

type config struct {
	urls       []string
	proxies    []string
	userAgent  string
	delay      time.Duration
	webhookUrl string
	notifyer   notify.Notifyer
}

var (
	cfg *config
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "nkmonitor",
	Short:   "nkmonitor is a monitor for nike.com.br",
	Long:    "nkmonitor is a configurable monitor for product restocks on nike.com.br",
	PreRunE: validateParams,
	RunE:    startMonitor,
}

func validateParams(cmd *cobra.Command, args []string) (err error) {
	defer func() {
		if err != nil {
			log.Error().Err(err).Msg("")
		}
	}()

	ua := useragent.Parse(cfg.userAgent)
	if ua.Name != "Chrome" || ua.Version == "" {
		return nkmonitor.ErrInvalidUserAgent
	}

	if cfg.delay < time.Second {
		return errors.New("delay too low")
	}

	if len(cfg.urls) == 0 {
		return errors.New("no urls")
	}

	for _, url := range cfg.urls {
		if _, err := nkmonitor.ParseNKUrl(url); err != nil {
			return fmt.Errorf("invalid url provided: %s", url)
		}
	}

	if cfg.webhookUrl == "" {
		cfg.notifyer = notify.NoopNotifyer{}
	} else {
		notifyer, err := notify.NewDiscordNotifyer(cfg.webhookUrl)
		if err != nil {
			return err
		}
		cfg.notifyer = notifyer
	}

	return nil
}

func startMonitor(cmd *cobra.Command, args []string) (err error) {
	log.Info().Msg("Starting monitor.")

	defer func() {
		if err != nil {
			log.Error().Err(err).Msg("")
		}
	}()

	m, _ := mimic.Chromium(mimic.BrandChrome, useragent.Parse(cfg.userAgent).Version)
	monitor, err := nkmonitor.NewMonitor(cfg.userAgent, cfg.delay, cfg.proxies, m)
	if err != nil {
		return err
	}

	err = monitor.Start()
	if err != nil {
		return err
	}

	log.Info().Msg("Monitor started successfully.")

	restockCh := make(chan nkmonitor.RestockInfo)

	go func() {
		for {
			info := <-restockCh
			log.Info().Str("product", info.Name).Msg("Restock found.")
			go cfg.notifyer.Notify(info)
		}
	}()

	log.Info().Msg("Adding urls.")
	for _, url := range cfg.urls {
		if _, err := monitor.AddTask(url, restockCh); err != nil {
			return err
		}
		log.Info().Str("url", url).Msg("Added.")
	}

	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	<-sigs

	log.Info().Msg("Stopping monitor.")
	return nil
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	cfg = &config{urls: make([]string, 1), proxies: make([]string, 0)}
	rootCmd.Flags().StringSliceVarP(&cfg.urls, "urls", "u", nil, "urls that will be fed to the monitor.")
	rootCmd.MarkFlagRequired("urls")
	rootCmd.Flags().StringSliceVarP(&cfg.proxies, "proxies", "p", nil, "HTTP proxies that will be used by the monitor. Uses localhost if none are provided.")
	rootCmd.Flags().StringVarP(&cfg.userAgent, "user-agent", "U", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/107.0.0.0 Safari/537.36", "user agent that will be used for monitoring, only Chrome UAs are currently supported")
	rootCmd.Flags().DurationVarP(&cfg.delay, "delay", "d", 8*time.Second, "time between requests (minimum 1s)")
	rootCmd.Flags().StringVarP(&cfg.webhookUrl, "webhook", "w", "", "discord webhook in url format")

}

func main() {
	Execute()
}
