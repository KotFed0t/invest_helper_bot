package model

import "github.com/govalues/decimal"

type Stock struct {
	Ticker       string
	Shortname    string
	Ordinal      int
	Lotsize      int
	TargetWeight decimal.Decimal
	ActualWeight decimal.Decimal
	Quantity     int
	Price        decimal.Decimal
	TotalPrice   decimal.Decimal
}
