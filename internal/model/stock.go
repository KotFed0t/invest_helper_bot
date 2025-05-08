package model

import "github.com/shopspring/decimal"

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

type StockChanges struct {
	Quantity        *int
	NewTargetWeight *decimal.Decimal
	CustomPrice     *decimal.Decimal
}

type StockOperation struct {
	Ticker     string
	Shortname  string
	Quantity   int
	Price      decimal.Decimal
	TotalPrice decimal.Decimal
	Currency   string
}
