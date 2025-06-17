package model

import (
	"github.com/shopspring/decimal"
)

type PortfolioPage struct {
	PortfolioSummary
	CurPage    int
	TotalPages int
	Stocks     []Stock
}

type PortfolioSummary struct {
	Portfolio
	BalanceInsideIndex        decimal.Decimal
	BalanceOutsideIndex       decimal.Decimal
	TotalWeight               decimal.Decimal
	StocksCount               int
	StocksOutsideIndexCnt     int
	IndexOffset               decimal.Decimal
	GrowthSumInsideIndex      decimal.Decimal
	GrowthSumOutsideIndex     decimal.Decimal
	GrowthPercentInsideIndex  decimal.Decimal
	GrowthPercentOutsideIndex decimal.Decimal
}

type Portfolio struct {
	PortfolioID   int64
	PortfolioName string
}

type PortfolioFullInfo struct {
	PortfolioSummary
	Stocks          []Stock
	StockOperations []StockOperation
}
