package telegram

import tele "gopkg.in/telebot.v4"

type InvestHelperService interface {
}

type Session interface {
}

type Controller struct {
	investHelperService InvestHelperService
	session             Session
}

func NewController(investHelperService InvestHelperService, session Session) *Controller {
	return &Controller{
		investHelperService: investHelperService,
		session:             session,
	}
}

func (ctrl *Controller) Start(c tele.Context) error {
	return c.Reply("Hello!")
}
