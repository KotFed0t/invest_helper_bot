package model

type action int

const (
	DefaultAction action = iota
	ExpectingPortfolioName
	ExpectingTicker
	ExpectingWeight
	ExpectingBuyStockQuantity
	ExpectingSellStockQuantity
	ExpectingChangePrice
	ExpectingPurchaseSum
)

type Session struct {
	Action                  action
	PortfolioID             int64
	StockTicker             string
	StockChanges            *StockChanges
	CurPortfolioListPage    int
	CurPortfolioDetailsPage int
	StocksToPurchase        []StockPurchase
}
