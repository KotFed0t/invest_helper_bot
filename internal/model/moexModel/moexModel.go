package moexModel

import "github.com/shopspring/decimal"

type RawStocksInfo struct {
	Securities Securities `json:"securities"`
	Marketdata Marketdata `json:"marketdata"`
}

type Securities struct {
	Columns []string `json:"columns"`
	Data    [][]any  `json:"data"`
}

type Marketdata struct {
	Columns []string `json:"columns"`
	Data    [][]any  `json:"data"`
}

type StockInfo struct {
	Ticker     string
	Shortname  string
	Lotsize    int
	CurrencyID string
	Status     bool
	Price      decimal.Decimal
}
