package config

import (
	"log"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	LogLevel          string `env:"LOG_LEVEL"`
	Postgres          Postgres
	Telegram          Telegram
	Redis             Redis
	API               API
	Cache             Cache
	Jobs              Jobs
	GoogleDrive       GoogleDrive
	SessionExpiration time.Duration `env:"SESSION_EXPIRATION"`
	StocksPerPage     int           `env:"STOCKS_PER_PAGE"`
	PortfoliosPerPage int           `env:"PORTFOLIOS_PER_PAGE"`
}

type Postgres struct {
	Host            string `env:"PG_HOST"`
	Port            int    `env:"PG_PORT"`
	DbName          string `env:"PG_DB_NAME"`
	Password        string `env:"PG_PASSWORD"`
	User            string `env:"PG_USER"`
	PoolMax         int    `env:"PG_POOL_MAX"`
	MaxOpenConns    int    `env:"PG_MAX_OPEN_CONNS"`
	ConnMaxLifetime int    `env:"PG_CONN_MAX_LIFETIME"`
	MaxIdleConns    int    `env:"PG_MAX_IDLE_CONNS"`
	ConnMaxIdleTime int    `env:"PG_CONN_MAX_IDLE_TIME"`
	MigrationDir    string `env:"PG_MIGRATION_DIR"`
}

type Telegram struct {
	Token            string        `env:"TELEGRAM_TOKEN"`
	UpdTimeout       time.Duration `env:"TELEGRAM_UPD_TIMEOUT"`
	FileLimitInBytes int           `env:"TELEGRAM_FILE_LIMIT_IN_BYTES"`
}

type Redis struct {
	Host     string `env:"REDIS_HOST"`
	Port     int    `env:"REDIS_PORT"`
	Password string `env:"REDIS_PASSWORD"`
	DB       int    `env:"REDIS_DB"`
}

type API struct {
	Debug   bool          `env:"API_DEBUG"`
	Timeout time.Duration `env:"API_TIMEOUT"`
	MoexApi MoexApi
}

type MoexApi struct {
	Url string `env:"MOEX_API_URL"`
}

type Cache struct {
	StocksExpiration time.Duration `env:"CACHE_STOCKS_EXPIRATION"`
}

type Jobs struct {
	FillMoexCacheInterval time.Duration `env:"FILL_MOEX_CACHE_JOB_INTERVAL"`
}

type GoogleDrive struct {
	CredentialsFile  string `env:"GOOGLE_DRIVE_CREDENTIALS_FILE"`
}

func MustLoad() *Config {
	_ = godotenv.Load(".env")

	cfg := &Config{}

	opts := env.Options{RequiredIfNoDef: true}

	if err := env.ParseWithOptions(cfg, opts); err != nil {
		log.Fatalf("parse config error: %s", err)
	}

	return cfg
}
