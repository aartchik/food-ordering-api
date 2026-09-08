package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type config struct {
	addr         string
	apiURL       string
	apiKey       string
	pollInterval time.Duration
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	var cfg config
	flag.StringVar(&cfg.addr, "addr", ":8081", "HTTP network address")
	flag.StringVar(&cfg.apiURL, "api-url", os.Getenv("KITCHEN_API_URL"), "Food Ordering API API URL")
	flag.StringVar(&cfg.apiKey, "api-key", os.Getenv("KITCHEN_API_KEY"), "Partner API key")
	flag.DurationVar(&cfg.pollInterval, "poll-interval", 2*time.Second, "Order polling interval")
	flag.Parse()
	if err := validateConfig(cfg); err != nil {
		return err
	}

	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)
	client := &kitchenClient{
		baseURL:    strings.TrimRight(cfg.apiURL, "/"),
		apiKey:     cfg.apiKey,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
	state := &serviceState{}
	w := &worker{api: client, state: state, interval: cfg.pollInterval, logger: errorLog}
	srv := &http.Server{
		Addr:         cfg.addr,
		Handler:      routes(state, errorLog),
		ErrorLog:     errorLog,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  time.Minute,
	}

	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithCancel(signalCtx)
	defer cancel()
	go w.run(ctx)

	serverError := make(chan error, 1)
	go func() {
		infoLog.Printf("starting demo restaurant on %s", srv.Addr)
		serverError <- srv.ListenAndServe()
	}()

	select {
	case err := <-serverError:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-signalCtx.Done():
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		if err := <-serverError; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		infoLog.Printf("stopped demo restaurant on %s", srv.Addr)
		return nil
	}
}

func validateConfig(cfg config) error {
	parsed, err := url.ParseRequestURI(cfg.apiURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("invalid kitchen API URL")
	}
	if strings.TrimSpace(cfg.apiKey) == "" {
		return fmt.Errorf("partner API key must be provided")
	}
	if cfg.pollInterval <= 0 {
		return fmt.Errorf("poll interval must be positive")
	}
	return nil
}
