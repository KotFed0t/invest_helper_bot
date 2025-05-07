package dbModel

import (
	"github.com/shopspring/decimal"
)

type Stock struct {
	PortfolioID int64           `db:"portfolio_id"`
	Ticker      string          `db:"ticker"`
	Weight      decimal.Decimal `db:"weight"`
	UserID      int64           `db:"user_id"`
	Quantity    int64           `db:"quantity"`
}
