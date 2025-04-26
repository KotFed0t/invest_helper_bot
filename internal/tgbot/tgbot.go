package tgbot

import (
	"log/slog"

	"github.com/KotFed0t/invest_helper_bot/config"
	"github.com/KotFed0t/invest_helper_bot/internal/transport/telegram"
	customMW "github.com/KotFed0t/invest_helper_bot/internal/transport/telegram/middleware"
	tele "gopkg.in/telebot.v4"
	"gopkg.in/telebot.v4/middleware"
)

type Session interface {
	// GetSession() error
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

	return &TGBot{bot: b}
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
		// получение сесии и выбор метода контроллера на основе шага пользователя и введенного текста
		return c.Reply("this is text")
	})

	b.bot.Handle("/start", b.ctrl.Start)

}
