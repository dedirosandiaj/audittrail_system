package config

import (
	"time"

	"github.com/caarlos0/env/v11"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	Env      string `env:"APP_ENV" envDefault:"development"`
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`

	HTTP struct {
		Addr            string        `env:"HTTP_ADDR" envDefault:":8080"`
		BodyLimitBytes  int           `env:"HTTP_BODY_LIMIT" envDefault:"262144"`
		ReadTimeout     time.Duration `env:"HTTP_READ_TIMEOUT" envDefault:"15s"`
		WriteTimeout    time.Duration `env:"HTTP_WRITE_TIMEOUT" envDefault:"15s"`
		ShutdownTimeout time.Duration `env:"HTTP_SHUTDOWN_TIMEOUT" envDefault:"10s"`
	}

	Postgres struct {
		URL             string        `env:"POSTGRES_URL,required"`
		MaxConns        int32         `env:"POSTGRES_MAX_CONNS" envDefault:"20"`
		MaxConnLifetime time.Duration `env:"POSTGRES_MAX_LIFETIME" envDefault:"30m"`
	}

	ClickHouse struct {
		Addr     string `env:"CLICKHOUSE_ADDR" envDefault:"clickhouse:9000"`
		Database string `env:"CLICKHOUSE_DB" envDefault:"auditrail"`
		User     string `env:"CLICKHOUSE_USER" envDefault:"default"`
		Password string `env:"CLICKHOUSE_PASSWORD" envDefault:""`
	}

	NATS struct {
		URL        string `env:"NATS_URL" envDefault:"nats://nats:4222"`
		StreamName string `env:"NATS_STREAM" envDefault:"EVENTS"`
	}

	Redis struct {
		Addr     string `env:"REDIS_ADDR" envDefault:"redis:6379"`
		Password string `env:"REDIS_PASSWORD" envDefault:""`
		DB       int    `env:"REDIS_DB" envDefault:"0"`
	}

	Security struct {
		MasterToken    string        `env:"ADMIN_MASTER_TOKEN,required"`
		MaxClockSkew   time.Duration `env:"MAX_CLOCK_SKEW" envDefault:"5m"`
		NonceTTL       time.Duration `env:"NONCE_TTL" envDefault:"10m"`
	}

	RateLimit struct {
		AuditPerMinute  int `env:"RATE_LIMIT_AUDIT" envDefault:"600"`
		ErrorPerMinute  int `env:"RATE_LIMIT_ERROR" envDefault:"1200"`
		LogPerMinute    int `env:"RATE_LIMIT_LOG" envDefault:"6000"`
		MetricPerMinute int `env:"RATE_LIMIT_METRIC" envDefault:"6000"`
	}
}

// Load parses environment variables into a Config.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
