package model

type action int

const (
	DefaultAction action = iota
	ExpectingPortfolioName
	ExpectingTicker
	ExpectingWeight
	ExpectingBuyStockQuantity
)

type Session struct {
	Action       action
	LastMsgId    int64
	PortfolioID  int64
	StockTicker  string
	StockChanges *StockChanges
}
