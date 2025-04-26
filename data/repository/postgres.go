package repository

import (
	"github.com/KotFed0t/invest_helper_bot/config"
	_ "github.com/jackc/pgx/stdlib" // pgx driver
	"github.com/jmoiron/sqlx"
)

type Postgres struct {
	db  *sqlx.DB
	cfg *config.Config
}

func NewPostgres(cfg *config.Config, db *sqlx.DB) *Postgres {
	return &Postgres{db: db, cfg: cfg}
}
