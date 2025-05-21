package tgbot

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	"github.com/KotFed0t/invest_helper_bot/config"
	"github.com/KotFed0t/invest_helper_bot/internal/model"
	"github.com/KotFed0t/invest_helper_bot/internal/model/tg/tgCallback.go"
	"github.com/KotFed0t/invest_helper_bot/internal/transport/telegram"
	customMW "github.com/KotFed0t/invest_helper_bot/internal/transport/telegram/middleware"
	"github.com/KotFed0t/invest_helper_bot/utils"
	tele "gopkg.in/telebot.v4"
	"gopkg.in/telebot.v4/middleware"
)

type Session interface {
	GetSession(ctx context.Context, key string) (model.Session, error)
	SetSession(ctx context.Context, key string, session model.Session) error
}

type TGBot struct {
	bot     *tele.Bot
	ctrl    *telegram.Controller
	session Session
}

func New(cfg *config.Config, ctrl *telegram.Controller, session Session) *TGBot {
	settings := tele.Settings{
		Token:  cfg.Telegram.Token,
		Poller: &tele.LongPoller{Timeout: cfg.Telegram.UpdTimeout},
	}

	b, err := tele.NewBot(settings)
	if err != nil {
		slog.Error("error while tele.NewBot", slog.String("err", err.Error()))
		panic(err)
	}

	return &TGBot{bot: b, ctrl: ctrl, session: session}
}

func (b *TGBot) Start() {
	b.bot.Use(middleware.Recover(), customMW.Logger())

	b.setupRoutes()

	go b.bot.Start()
	slog.Info("tgbot started!")
}

func (b *TGBot) Stop() {
	slog.Info("start stopping tgbot")
	b.bot.Stop()
	slog.Info("tgbot stopped")
}

func (b *TGBot) setupRoutes() {
	// commands
	b.bot.Handle("/start", b.ctrl.Start)
	b.bot.Handle("/create_stocks_portfolio", b.ctrl.InitStocksPortfolioCreation)
	b.bot.Handle("/my_portfolios", b.ctrl.GetPortfolios)

	// text
	b.bot.Handle(tele.OnText, func(c tele.Context) error {
		// получение сесии и выбор метода контроллера на основе шага пользователя
		ctx := utils.CreateCtxWithRqID(c)
		rqID := utils.GetRequestIDFromCtx(ctx)
		chatSession, err := b.session.GetSession(ctx, strconv.FormatInt(c.Chat().ID, 10))
		if err != nil {
			slog.Error("got error from session.GetSession", slog.String("rqID", rqID), slog.String("err", err.Error()))
			return c.Send("что-то пошло не так...")
		}

		c.Set("session", chatSession)

		switch chatSession.Action {
		case model.ExpectingPortfolioName:
			return b.ctrl.ProcessStocksPortfolioCreation(c)
		case model.ExpectingTicker:
			return b.ctrl.ProcessAddStock(c)
		case model.ExpectingWeight:
			return b.ctrl.ProcessChangeWeight(c)
		case model.ExpectingBuyStockQuantity:
			return b.ctrl.ProcessBuyStock(c)
		case model.ExpectingSellStockQuantity:
			return b.ctrl.ProcessSellStock(c)
		case model.ExpectingChangePrice:
			return b.ctrl.ProcessChangePrice(c)
		case model.ExpectingPurchaseSum:
			return b.ctrl.ProcessCalculatePurchase(c)
		default:
			slog.Error("unexpected chatSession action", slog.String("rqID", rqID), slog.Any("state", chatSession.Action))
			return c.Send("сначала введите одну из команд")
		}
	})

	// callbacks
	b.bot.Handle(tele.OnCallback, func(c tele.Context) error {
		callbackBtnText := strings.TrimPrefix(c.Callback().Data, "\f")

		switch {
		case callbackBtnText == tgCallback.AddStock:
			return b.ctrl.InitAddStock(c)
		case callbackBtnText == tgCallback.ChangeWeight:
			return b.ctrl.InitChangeWeight(c)
		case callbackBtnText == tgCallback.BuyStock:
			return b.ctrl.InitBuyStock(c)
		case callbackBtnText == tgCallback.SellStock:
			return b.ctrl.InitSellStock(c)
		case callbackBtnText == tgCallback.ChangePrice:
			return b.ctrl.InitChangePrice(c)
		case callbackBtnText == tgCallback.SaveStockChanges:
			return b.ctrl.SaveStockChanges(c)
		case callbackBtnText == tgCallback.AddStockToPortfolio:
			return b.ctrl.ProcessAddStockToPortfolio(c)
		case callbackBtnText == tgCallback.DeleteStock:
			return b.ctrl.ProcessDeleteStock(c)
		case callbackBtnText == tgCallback.BackToPortolio:
			return b.ctrl.ProcessBackToPortfolio(c)
		case callbackBtnText == tgCallback.CalculatePurchase:
			return b.ctrl.InitCalculatePurchase(c)
		case callbackBtnText == tgCallback.BackToPortolioList:
			return b.ctrl.ProcessBackToPortfolioList(c)
		case callbackBtnText == tgCallback.RebalanceWeights:
			return b.ctrl.RebalanceWeights(c)
		case callbackBtnText == tgCallback.InitDeletePortfolio:
			return b.ctrl.InitDeletePortfolio(c)
		case callbackBtnText == tgCallback.ProcessDeletePortfolio:
			return b.ctrl.ProcessDeletePortfolio(c)
		case callbackBtnText == tgCallback.GenerateReport:
			return b.ctrl.GenerateReport(c)
		case callbackBtnText == tgCallback.ApplyCalculatedPurchaseToPortfolio:
			return b.ctrl.ApplyCalculatedPurchaseToPortfolio(c)
		case callbackBtnText == tgCallback.PageNumber:
			return nil
		case strings.HasPrefix(callbackBtnText, tgCallback.EditStockPrefix):
			return b.ctrl.GoToEditStock(c)
		case strings.HasPrefix(callbackBtnText, tgCallback.ToPortfolioPage):
			return b.ctrl.GoToPortfolioPage(c)
		case strings.HasPrefix(callbackBtnText, tgCallback.ToPortfolioListPage):
			return b.ctrl.GetPortfolios(c)
		case strings.HasPrefix(callbackBtnText, tgCallback.EditPortfolioPrefix):
			return b.ctrl.GoToEditPortfolio(c)
		default:
			return c.Send("callback не опознан")
		}
	})

}
