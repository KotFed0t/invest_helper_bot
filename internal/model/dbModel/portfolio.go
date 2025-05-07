package dbModel

import (
	"github.com/shopspring/decimal"
)

type Portfolio struct {
	weight decimal.Decimal `db:"weight"`
}
