package tgbot

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/KotFed0t/invest_helper_bot/config"
	"github.com/KotFed0t/invest_helper_bot/internal/model"
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

		switch chatSession.State {
		case model.ExpectingPortfolioName:
			return b.ctrl.CreateStocksPortfolio(c)
		default:
			slog.Error("unexpected chatSession state", slog.String("rqID", rqID), slog.Any("state", chatSession.State))
			return c.Send("сначала введите одну из команд")
		}
	})

	b.bot.Handle("/start", b.ctrl.Start)

	b.bot.Handle("/create_stocks_portfolio", b.ctrl.StartStocksPortfolioCreation)

}
