package dbModel

import (
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/govalues/decimal"
)

type Portfolio struct {
	weight decimal.Decimal `db:"weight"`
	id pgtype.Int8 `db:"id"`
}
