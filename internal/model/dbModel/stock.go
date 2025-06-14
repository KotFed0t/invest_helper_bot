package dbModel

import (
	"time"

	"github.com/shopspring/decimal"
)

type Stock struct {
	PortfolioID int64           `db:"portfolio_id"`
	Ticker      string          `db:"ticker"`
	Weight      decimal.Decimal `db:"weight"`
	Quantity    int             `db:"quantity"`
}

type StockOperation struct {
	PortfolioID int64           `db:"portfolio_id"`
	Ticker      string          `db:"ticker"`
	Shortname   string          `db:"shortname"`
	Quantity    int             `db:"quantity"`
	Price       decimal.Decimal `db:"price"`
	TotalPrice  decimal.Decimal `db:"total_price"`
	Currency    string          `db:"currency"`
	DtCreate    time.Time       `db:"dt_create"`
}

type StockRemaining struct {
	RowID       int64           `db:"row_id"`
	PortfolioID int64           `db:"portfolio_id"`
	Ticker      string          `db:"ticker"`
	Quantity    int             `db:"quantity"`
	Price       decimal.Decimal `db:"price"`
	DtCreate    time.Time       `db:"dt_create"`
	DtUpdate    time.Time       `db:"dt_update"`
}
