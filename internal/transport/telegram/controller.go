package telegram

import (
	"context"
	"errors"
	"log/slog"
	"strconv"

	"github.com/KotFed0t/invest_helper_bot/data/session"
	"github.com/KotFed0t/invest_helper_bot/internal/model"
	"github.com/KotFed0t/invest_helper_bot/utils"
	tele "gopkg.in/telebot.v4"
)

type InvestHelperService interface {
	RegUser(ctx context.Context, chatID int64) error
	CreateStocksPortfolio(ctx context.Context, portfolioName string, chatID int64) error
}

type Session interface {
	GetSession(ctx context.Context, key string) (model.Session, error)
	SetSession(ctx context.Context, key string, session model.Session) error
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
	ctx := utils.CreateCtxWithRqID(c)
	_ = ctrl.investHelperService.RegUser(ctx, c.Chat().ID)
	return c.Reply("Hello!")
}

func (ctrl *Controller) StartStocksPortfolioCreation(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	strChatID := strconv.FormatInt(c.Chat().ID, 10)
	// получить сессию и установить ожидание ввода названия портфеля
	chatSession, err := ctrl.session.GetSession(ctx, strChatID)
	if err != nil && !errors.Is(err, session.ErrNotFound) {
		slog.Error("got error from session.GetSession", slog.String("rqID", rqID), slog.String("err", err.Error()))
		return c.Send("что-то пошло не так...")
	}

	chatSession.State = model.ExpectingPortfolioName
	err = ctrl.session.SetSession(ctx, strChatID, chatSession)
	if err != nil {
		slog.Error("got error from session.SetSession", slog.String("rqID", rqID), slog.String("err", err.Error()))
		return c.Send("что-то пошло не так...")
	}

	return c.Send("Введите название портфеля:")
}

func (ctrl *Controller) CreateStocksPortfolio(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		return c.Send(internalErrMsg)
	}

	defer func() {
		chatSession.State = model.DefaultState
		_ = ctrl.session.SetSession(ctx, strconv.FormatInt(c.Chat().ID, 10), chatSession)
	}()

	err = ctrl.investHelperService.CreateStocksPortfolio(ctx, c.Message().Text, c.Chat().ID)
	if err != nil {
		slog.Error("got error from investHelperService.CreateStocksPortfolio", slog.String("rqID", rqID), slog.String("err", err.Error()))
		return c.Send(internalErrMsg)
	}

	return c.Send("портфель успешно создан")
}

func (ctrl *Controller) getSessionFromTeleCtxOrStorage(ctx context.Context, c tele.Context) (model.Session, error) {
	chatSession, ok := c.Get("session").(model.Session)
	if ok {
		return chatSession, nil
	}

	rqID := utils.GetRequestIDFromCtx(ctx)
	chatSession, err := ctrl.session.GetSession(ctx, strconv.FormatInt(c.Chat().ID, 10))
	if err != nil {
		if !errors.Is(err, session.ErrNotFound) {
			slog.Error("got error from session.GetSession", slog.String("rqID", rqID), slog.String("err", err.Error()))
		}
		return model.Session{}, err
	}
	return chatSession, nil
}
