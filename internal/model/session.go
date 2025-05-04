package model

type state int

const (
	DefaultState state = iota
	ExpectingPortfolioName
	ExpectingTicker
)

type Session struct {
	State       state
	LastMsgId   int64
	PortfolioID int64
}
