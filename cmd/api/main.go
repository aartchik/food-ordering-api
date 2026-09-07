package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"
)

type config struct {
	addr string
	env  string
}

type application struct {
	config   config
	errorLog *log.Logger
	infoLog  *log.Logger
}

func main() {
	var cfg config

	flag.StringVar(&cfg.addr, "addr", ":8080", "HTTP network address")
	flag.StringVar(&cfg.env, "env", "development", "Environment")
	flag.Parse()

	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

	app := &application{
		config:   cfg,
		errorLog: errorLog,
		infoLog:  infoLog,
	}

	srv := &http.Server{
		Addr:         cfg.addr,
		Handler:      app.routes(),
		ErrorLog:     errorLog,
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	infoLog.Printf("starting server on %s", cfg.addr)
	err := srv.ListenAndServe()
	errorLog.Fatal(err)
}
