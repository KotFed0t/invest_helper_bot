package data

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/KotFed0t/invest_helper_bot/config"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/stdlib"
	"github.com/jmoiron/sqlx"
)

const (
	defaultConnAttemts = 10
	connTimeout        = time.Second
)

func NewPostgresClient(cfg *config.Config) *sqlx.DB {
	dataSourceName := fmt.Sprintf("host=%s port=%d user=%s dbname=%s sslmode=disable password=%s",
		cfg.Postgres.Host,
		cfg.Postgres.Port,
		cfg.Postgres.User,
		cfg.Postgres.DbName,
		cfg.Postgres.Password,
	)

	connAttempts := defaultConnAttemts
	var db *sqlx.DB
	var err error

	for connAttempts > 0 {
		db, err = sqlx.Connect("pgx", dataSourceName)
		if err == nil {
			break
		}

		slog.Info("Postgres is trying to connect", slog.Int("attempts left", connAttempts))

		time.Sleep(connTimeout)

		connAttempts--
	}

	if err != nil {
		slog.Error("Postgres connAttempts = 0")
		panic(err)
	}

	db.SetMaxOpenConns(cfg.Postgres.MaxOpenConns)
	db.SetConnMaxLifetime(time.Duration(cfg.Postgres.ConnMaxLifetime) * time.Second)
	db.SetMaxIdleConns(cfg.Postgres.MaxIdleConns)
	db.SetConnMaxIdleTime(time.Duration(cfg.Postgres.ConnMaxIdleTime) * time.Second)
	if err = db.Ping(); err != nil {
		slog.Error("Postgres dbPing error")
		panic(err)
	}
	slog.Info("Postgres connected")

	migratePostgres(db, cfg.Postgres.MigrationDir)
	slog.Info("postgres migrated successfully")

	return db
}

func migratePostgres(db *sqlx.DB, migrationDir string) {
	driver, err := postgres.WithInstance(db.DB, &postgres.Config{})
	if err != nil {
		slog.Error("postgres migration failed on postgres.WithInstance", slog.String("err", err.Error()))
		panic(err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationDir),
		"postgres",
		driver,
	)
	if err != nil {
		slog.Error("postgres migration failed on migrate.NewWithDatabaseInstance", slog.String("err", err.Error()))
		panic(err)
	}

	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		slog.Error("postgres migration failed on m.Up()", slog.String("err", err.Error()))
		panic(err)
	}
}
