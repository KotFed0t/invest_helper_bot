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
)

type Session struct {
	Action       action
	LastMsgId    int64
	PortfolioID  int64
	StockTicker  string
	StockChanges *StockChanges
}
