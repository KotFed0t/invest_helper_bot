package model

type state int

const (
	DefaultState state = iota
	ExpectingPortfolioName
)

type Session struct {
	State             state
	LastMsgId         int
}
