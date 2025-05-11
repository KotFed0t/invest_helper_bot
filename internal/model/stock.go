package model

import "github.com/shopspring/decimal"

type Stock struct {
	StockBase
	Shortname    string
	Lotsize      int
	ActualWeight decimal.Decimal
	Price        decimal.Decimal
	TotalPrice   decimal.Decimal
}

type StockBase struct {
	PortfolioID  int64
	Ticker       string
	TargetWeight decimal.Decimal
	Quantity     int
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

type StockPurchase struct {
	Ticker       string
	Shortname    string
	LotSize      int
	LotsQuantity decimal.Decimal
	StockPrice   decimal.Decimal
}
