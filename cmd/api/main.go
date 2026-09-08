package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"food-ordering-api/internal/cache"
	"food-ordering-api/internal/models"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

type config struct {
	addr string
	env  string
	db   struct {
		dsn          string
		maxOpenConns int
		maxIdleConns int
		maxIdleTime  time.Duration
	}
	limiter struct {
		rps     float64
		burst   int
		enabled bool
	}
	redis struct {
		addr         string
		password     string
		db           int
		dialTimeout  time.Duration
		cacheTimeout time.Duration
		menuTTL      time.Duration
		enabled      bool
	}
	partnerKeys string
}

type application struct {
	config    config
	errorLog  *log.Logger
	infoLog   *log.Logger
	models    models.Models
	partners  partnerAuthenticator
	menuCache *cache.MenuCache
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	var cfg config

	flag.StringVar(&cfg.addr, "addr", ":8080", "HTTP network address")
	flag.StringVar(&cfg.env, "env", "development", "Environment")
	flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("FOOD_ORDERING_API_DB_DSN"), "PostgreSQL DSN")
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
	flag.DurationVar(&cfg.db.maxIdleTime, "db-max-idle-time", 15*time.Minute, "PostgreSQL max connection idle time")
	flag.Float64Var(&cfg.limiter.rps, "limiter-rps", 2, "Rate limiter maximum requests per second")
	flag.IntVar(&cfg.limiter.burst, "limiter-burst", 4, "Rate limiter maximum burst")
	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")
	flag.StringVar(&cfg.redis.addr, "redis-addr", envOrDefault("FOOD_ORDERING_API_REDIS_ADDR", "localhost:6379"), "Redis server address")
	flag.StringVar(&cfg.redis.password, "redis-password", os.Getenv("FOOD_ORDERING_API_REDIS_PASSWORD"), "Redis password")
	flag.IntVar(&cfg.redis.db, "redis-db", 0, "Redis database number")
	flag.DurationVar(&cfg.redis.dialTimeout, "redis-dial-timeout", 2*time.Second, "Redis connection timeout")
	flag.DurationVar(&cfg.redis.cacheTimeout, "redis-cache-timeout", 200*time.Millisecond, "Redis cache operation timeout")
	flag.DurationVar(&cfg.redis.menuTTL, "menu-cache-ttl", 5*time.Minute, "Menu cache TTL")
	flag.BoolVar(&cfg.redis.enabled, "redis-enabled", true, "Enable Redis cache")
	flag.StringVar(&cfg.partnerKeys, "partner-keys", os.Getenv("FOOD_ORDERING_API_PARTNER_KEYS"), "Comma-separated partner_id=api_key pairs")
	flag.Parse()

	partners, err := newConfiguredPartners(cfg.partnerKeys)
	if err != nil {
		return err
	}

	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

	db, err := openDB(cfg)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			errorLog.Printf("close database: %v", err)
		}
	}()

	infoLog.Println("database connection pool established")

	redisClient, err := openRedis(cfg)
	if err != nil {
		errorLog.Printf("redis unavailable, cache disabled: %v", err)
	}
	if redisClient != nil {
		defer func() {
			if err := redisClient.Close(); err != nil {
				errorLog.Printf("close redis client: %v", err)
			}
		}()
		infoLog.Println("redis connection established")
	}

	app := &application{
		config:    cfg,
		errorLog:  errorLog,
		infoLog:   infoLog,
		models:    models.NewModels(db),
		partners:  partners,
		menuCache: cache.NewMenuCache(redisClient, cfg.redis.menuTTL, cfg.redis.cacheTimeout),
	}

	return app.serve()
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func openRedis(cfg config) (*redis.Client, error) {
	if !cfg.redis.enabled {
		return nil, nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:         cfg.redis.addr,
		Password:     cfg.redis.password,
		DB:           cfg.redis.db,
		DialTimeout:  cfg.redis.dialTimeout,
		ReadTimeout:  cfg.redis.cacheTimeout,
		WriteTimeout: cfg.redis.cacheTimeout,
		PoolTimeout:  cfg.redis.cacheTimeout,
	})

	ctx, cancel := context.WithTimeout(context.Background(), cfg.redis.dialTimeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		if closeErr := client.Close(); closeErr != nil {
			return nil, errors.Join(err, fmt.Errorf("close redis client after failed ping: %w", closeErr))
		}
		return nil, err
	}

	return client, nil
}

func newConfiguredPartners(value string) (configuredPartners, error) {
	partners := configuredPartners{}
	if strings.TrimSpace(value) == "" {
		return partners, nil
	}

	for _, pair := range strings.Split(value, ",") {
		partnerID, key, found := strings.Cut(pair, "=")
		partnerID = strings.TrimSpace(partnerID)
		key = strings.TrimSpace(key)
		if !found || partnerID == "" || key == "" {
			return nil, fmt.Errorf("invalid partner key configuration")
		}
		if _, exists := partners[partnerID]; exists {
			return nil, fmt.Errorf("duplicate partner id %q", partnerID)
		}
		partners[partnerID] = apiKeyHash(key)
	}

	return partners, nil
}

func openDB(cfg config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.db.dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(cfg.db.maxOpenConns)
	db.SetMaxIdleConns(cfg.db.maxIdleConns)
	db.SetConnMaxIdleTime(cfg.db.maxIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, errors.Join(err, fmt.Errorf("close database after failed ping: %w", closeErr))
		}
		return nil, err
	}

	return db, nil
}
