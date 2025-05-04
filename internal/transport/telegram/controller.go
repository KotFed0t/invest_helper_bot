package telegram

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/KotFed0t/invest_helper_bot/data/session"
	"github.com/KotFed0t/invest_helper_bot/internal/converter/telebotConverter"
	"github.com/KotFed0t/invest_helper_bot/internal/model"
	"github.com/KotFed0t/invest_helper_bot/internal/service"
	"github.com/KotFed0t/invest_helper_bot/utils"
	tele "gopkg.in/telebot.v4"
)

type InvestHelperService interface {
	RegUser(ctx context.Context, chatID int64) error
	CreateStocksPortfolio(ctx context.Context, portfolioName string, chatID int64) (portfolioID int64, err error)
	GetStockInfo(ctx context.Context, ticker string) (model.Stock, error)
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

func (ctrl *Controller) InitStocksPortfolioCreation(c tele.Context) error {
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

func (ctrl *Controller) ProcessStocksPortfolioCreation(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	var portfolioID int64

	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		return c.Send(internalErrMsg)
	}

	defer func() {
		chatSession.State = model.DefaultState
		chatSession.PortfolioID = portfolioID
		_ = ctrl.session.SetSession(ctx, strconv.FormatInt(c.Chat().ID, 10), chatSession)
	}()

	portfolioID, err = ctrl.investHelperService.CreateStocksPortfolio(ctx, c.Message().Text, c.Chat().ID)
	if err != nil {
		slog.Error("got error from investHelperService.CreateStocksPortfolio", slog.String("rqID", rqID), slog.String("err", err.Error()))
		return c.Send(internalErrMsg)
	}

	portfolio := model.Portfolio{Name: c.Message().Text}
	return c.Send(telebotConverter.PortfolioDetailsResponse(portfolio))
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

func (ctrl *Controller) InitAddStock(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		return c.Send(internalErrMsg)
	}

	chatSession.State = model.ExpectingTicker
	err = ctrl.session.SetSession(ctx, strconv.FormatInt(c.Chat().ID, 10), chatSession)
	if err != nil {
		return c.Send(internalErrMsg)
	}

	return c.Edit("Введите тикер")
}

func (ctrl *Controller) ProcessAddStock(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	// chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	// if err != nil {
	// 	return c.Send(internalErrMsg)
	// }

	ticker := strings.ToUpper(c.Message().Text)

	stock, err := ctrl.investHelperService.GetStockInfo(ctx, ticker)
	slog.Info("stock", slog.Any("stock", stock))
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return c.Send("Не удалось найти указанный тикер")
		}
		if errors.Is(err, service.ErrStockNotActive) {
			return c.Send("акция не торгуется")
		}
		return c.Send(internalErrMsg)
	}
	return c.Send("нашли")
}
