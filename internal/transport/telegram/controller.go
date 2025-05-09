package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/KotFed0t/invest_helper_bot/data/session"
	"github.com/KotFed0t/invest_helper_bot/internal/converter/telebotConverter"
	"github.com/KotFed0t/invest_helper_bot/internal/model"
	"github.com/KotFed0t/invest_helper_bot/internal/model/moexModel"
	"github.com/KotFed0t/invest_helper_bot/internal/service"
	"github.com/KotFed0t/invest_helper_bot/utils"
	"github.com/shopspring/decimal"
	tele "gopkg.in/telebot.v4"
)

type InvestHelperService interface {
	RegUser(ctx context.Context, chatID int64) error
	CreateStocksPortfolio(ctx context.Context, portfolioName string, chatID int64) (portfolioID int64, err error)
	GetStockInfo(ctx context.Context, ticker string) (stockInfo moexModel.StockInfo, err error)
	GetPortfolioStockInfo(ctx context.Context, ticker string, portfolioID int64) (model.Stock, error)
	AddStockToPortfolio(ctx context.Context, ticker string, portfolioID, chatID int64) (model.Stock, error)
	SaveStockChangesToPortfolio(ctx context.Context, portfolioID int64, ticker string, weight *decimal.Decimal, quantity *int, price *decimal.Decimal) (model.Stock, error)
	DeleteStockFromPortfolio(ctx context.Context, portfolioID int64, ticker string) (model.PortfolioPage, error)
	GetPortfolioPage(ctx context.Context, portfolioID int64, page int) (model.PortfolioPage, error)
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
	go ctrl.investHelperService.RegUser(context.WithoutCancel(ctx), c.Chat().ID)
	return c.Reply("Hello!")
}

func (ctrl *Controller) InitStocksPortfolioCreation(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.InitStocksPortfolioCreation"
	strChatID := strconv.FormatInt(c.Chat().ID, 10)
	// получить сессию и установить ожидание ввода названия портфеля
	chatSession, err := ctrl.session.GetSession(ctx, strChatID)
	if err != nil && !errors.Is(err, session.ErrNotFound) {
		slog.Error("got error from session.GetSession", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return c.Send("что-то пошло не так...")
	}

	chatSession.Action = model.ExpectingPortfolioName
	err = ctrl.session.SetSession(ctx, strChatID, chatSession)
	if err != nil {
		slog.Error("got error from session.SetSession", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return c.Send("что-то пошло не так...")
	}

	return c.Send("Введите название портфеля:")
}

func (ctrl *Controller) ProcessStocksPortfolioCreation(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.ProcessStocksPortfolioCreation"

	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		return c.Send(internalErrMsg)
	}

	defer func() {
		chatSession.Action = model.DefaultAction
		go ctrl.session.SetSession(ctx, strconv.FormatInt(c.Chat().ID, 10), chatSession)
	}()

	portfolioID, err := ctrl.investHelperService.CreateStocksPortfolio(ctx, c.Message().Text, c.Chat().ID)
	if err != nil {
		slog.Error("got error from investHelperService.CreateStocksPortfolio", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return c.Send(internalErrMsg)
	}

	chatSession.PortfolioID = portfolioID

	portfolio := model.PortfolioPage{
		PortfolioSummary: model.PortfolioSummary{
			PortfolioName: c.Message().Text,
		},
	}
	return c.Send(telebotConverter.PortfolioDetailsResponse(portfolio))
}

func (ctrl *Controller) getSessionFromTeleCtxOrStorage(ctx context.Context, c tele.Context) (model.Session, error) {
	op := "Controller.getSessionFromTeleCtxOrStorage"
	chatSession, ok := c.Get("session").(model.Session)
	if ok {
		return chatSession, nil
	}

	rqID := utils.GetRequestIDFromCtx(ctx)
	chatSession, err := ctrl.session.GetSession(ctx, strconv.FormatInt(c.Chat().ID, 10))
	if err != nil {
		if !errors.Is(err, session.ErrNotFound) {
			slog.Error("got error from session.GetSession", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
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

	chatSession.Action = model.ExpectingTicker
	err = ctrl.session.SetSession(ctx, strconv.FormatInt(c.Chat().ID, 10), chatSession)
	if err != nil {
		return c.Send(internalErrMsg)
	}

	return c.Edit("Введите тикер")
}

func (ctrl *Controller) ProcessAddStock(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		return c.Send(internalErrMsg)
	}

	defer func() {
		chatSession.Action = model.DefaultAction
		go ctrl.session.SetSession(ctx, strconv.FormatInt(c.Chat().ID, 10), chatSession)
	}()

	ticker := strings.ToUpper(c.Message().Text)

	stockInfo, err := ctrl.investHelperService.GetStockInfo(ctx, ticker)
	if err != nil {
		// TODO мб убрать кнопку ввести тикер повторно, когда ввел несуществующий?
		if errors.Is(err, service.ErrNotFound) {
			return c.Send("Не удалось найти указанный тикер", telebotConverter.StockNotFoundMarkup())
		}
		if errors.Is(err, service.ErrStockNotActive) {
			return c.Send("акция не торгуется", telebotConverter.StockNotFoundMarkup())
		}
		return c.Send(internalErrMsg)
	}

	chatSession.StockTicker = ticker

	return c.Send(telebotConverter.StockAddResponse(stockInfo))
}

func (ctrl *Controller) InitChangeWeight(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		return c.Send(internalErrMsg)
	}

	chatSession.Action = model.ExpectingWeight
	err = ctrl.session.SetSession(ctx, strconv.FormatInt(c.Chat().ID, 10), chatSession)
	if err != nil {
		return c.Send(internalErrMsg)
	}

	return c.Edit("введите новое значение веса:")
}

func (ctrl *Controller) ProcessChangeWeight(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.ProcessChangeWeight"
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		return c.Send(internalErrMsg)
	}

	input := strings.Replace(c.Message().Text, ",", ".", 1)

	weight, err := decimal.NewFromString(input)
	if err != nil || weight.IsNegative() {
		return c.Send("Вес должен быть положительным числом, введите корректное значение:")
	}

	defer func() {
		chatSession.Action = model.DefaultAction
		go ctrl.session.SetSession(ctx, strconv.FormatInt(c.Chat().ID, 10), chatSession)
	}()

	if chatSession.StockTicker == "" {
		slog.Error("stockTicker is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return c.Send(internalErrMsg)
	}

	if chatSession.PortfolioID == 0 {
		slog.Error("PortfolioID is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return c.Send(internalErrMsg)
	}

	stock, err := ctrl.investHelperService.GetPortfolioStockInfo(ctx, chatSession.StockTicker, chatSession.PortfolioID)
	if err != nil {
		slog.Error("failed on investHelperService.GetPortfolioStockInfo", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return c.Send(internalErrMsg)
	}

	if chatSession.StockChanges != nil {
		chatSession.StockChanges.NewTargetWeight = &weight
	} else {
		chatSession.StockChanges = &model.StockChanges{NewTargetWeight: &weight}
	}

	return c.Send(telebotConverter.StockDetailResponse(stock, chatSession.StockChanges))
}

func (ctrl *Controller) InitBuyStock(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		return c.Send(internalErrMsg)
	}

	chatSession.Action = model.ExpectingBuyStockQuantity
	err = ctrl.session.SetSession(ctx, strconv.FormatInt(c.Chat().ID, 10), chatSession)
	if err != nil {
		return c.Send(internalErrMsg)
	}

	return c.Edit("введите кол-во акций к покупке:")
}

func (ctrl *Controller) ProcessBuyStock(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.ProcessBuyStock"
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		return c.Send(internalErrMsg)
	}

	quantity, err := strconv.Atoi(c.Message().Text)
	if err != nil || quantity <= 0 {
		return c.Send("количество должно быть целым числом больше 0, введите корректное значение:")
	}

	defer func() {
		chatSession.Action = model.DefaultAction
		go ctrl.session.SetSession(ctx, strconv.FormatInt(c.Chat().ID, 10), chatSession)
	}()

	if chatSession.StockTicker == "" {
		slog.Error("stockTicker is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return c.Send(internalErrMsg)
	}

	if chatSession.PortfolioID == 0 {
		slog.Error("PortfolioID is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return c.Send(internalErrMsg)
	}

	stock, err := ctrl.investHelperService.GetPortfolioStockInfo(ctx, chatSession.StockTicker, chatSession.PortfolioID)
	if err != nil {
		slog.Error("failed on investHelperService.GetPortfolioStockInfo", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return c.Send(internalErrMsg)
	}

	if chatSession.StockChanges != nil {
		chatSession.StockChanges.Quantity = &quantity
	} else {
		chatSession.StockChanges = &model.StockChanges{Quantity: &quantity}
	}

	return c.Send(telebotConverter.StockDetailResponse(stock, chatSession.StockChanges))
}

func (ctrl *Controller) InitSellStock(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		return c.Send(internalErrMsg)
	}

	chatSession.Action = model.ExpectingSellStockQuantity
	err = ctrl.session.SetSession(ctx, strconv.FormatInt(c.Chat().ID, 10), chatSession)
	if err != nil {
		return c.Send(internalErrMsg)
	}

	return c.Edit("введите кол-во акций к продаже:")
}

func (ctrl *Controller) ProcessSellStock(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.ProcessSellStock"
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		return c.Send(internalErrMsg)
	}

	quantity, err := strconv.Atoi(c.Message().Text)
	if err != nil || quantity <= 0 {
		return c.Send("количество должно быть целым числом больше 0, введите корректное значение:")
	}

	if chatSession.StockTicker == "" {
		slog.Error("stockTicker is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return c.Send(internalErrMsg)
	}

	if chatSession.PortfolioID == 0 {
		slog.Error("PortfolioID is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return c.Send(internalErrMsg)
	}

	stock, err := ctrl.investHelperService.GetPortfolioStockInfo(ctx, chatSession.StockTicker, chatSession.PortfolioID)
	if err != nil {
		slog.Error("failed on investHelperService.GetPortfolioStockInfo", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return c.Send(internalErrMsg)
	}

	if stock.Quantity < quantity {
		return c.Send(fmt.Sprintf("нельзя продать больше, чем есть в портфеле (%d шт). Введите корректное значение:", stock.Quantity))
	}

	sellQuantity := quantity * -1
	if chatSession.StockChanges != nil {
		chatSession.StockChanges.Quantity = &sellQuantity
	} else {
		chatSession.StockChanges = &model.StockChanges{Quantity: &sellQuantity}
	}

	chatSession.Action = model.DefaultAction
	go ctrl.session.SetSession(ctx, strconv.FormatInt(c.Chat().ID, 10), chatSession)

	return c.Send(telebotConverter.StockDetailResponse(stock, chatSession.StockChanges))
}

func (ctrl *Controller) InitChangePrice(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		return c.Send(internalErrMsg)
	}

	chatSession.Action = model.ExpectingChangePrice
	err = ctrl.session.SetSession(ctx, strconv.FormatInt(c.Chat().ID, 10), chatSession)
	if err != nil {
		return c.Send(internalErrMsg)
	}

	return c.Edit("введите цену за 1 акцию:")
}

func (ctrl *Controller) ProcessChangePrice(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.ProcessChangePrice"
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		return c.Send(internalErrMsg)
	}

	input := strings.Replace(c.Message().Text, ",", ".", 1)

	price, err := decimal.NewFromString(input)
	if err != nil || price.IsNegative() || price.IsZero() {
		return c.Send("цена должна быть числом больше 0, введите корректное значение:")
	}

	if chatSession.StockTicker == "" {
		slog.Error("stockTicker is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return c.Send(internalErrMsg)
	}

	if chatSession.PortfolioID == 0 {
		slog.Error("PortfolioID is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return c.Send(internalErrMsg)
	}

	stock, err := ctrl.investHelperService.GetPortfolioStockInfo(ctx, chatSession.StockTicker, chatSession.PortfolioID)
	if err != nil {
		slog.Error("failed on investHelperService.GetPortfolioStockInfo", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return c.Send(internalErrMsg)
	}

	if chatSession.StockChanges != nil {
		chatSession.StockChanges.CustomPrice = &price
	} else {
		chatSession.StockChanges = &model.StockChanges{CustomPrice: &price}
	}

	chatSession.Action = model.DefaultAction
	go ctrl.session.SetSession(ctx, strconv.FormatInt(c.Chat().ID, 10), chatSession)

	return c.Send(telebotConverter.StockDetailResponse(stock, chatSession.StockChanges))
}

func (ctrl *Controller) ProcessAddStockToPortfolio(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.ProcessAddStockToPortfolio"
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		return c.Send(internalErrMsg)
	}

	if chatSession.StockTicker == "" {
		slog.Error("stockTicker is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return c.Send(internalErrMsg)
	}

	if chatSession.PortfolioID == 0 {
		slog.Error("PortfolioID is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return c.Send(internalErrMsg)
	}

	stock, err := ctrl.investHelperService.AddStockToPortfolio(ctx, chatSession.StockTicker, chatSession.PortfolioID, c.Chat().ID)
	if err != nil {
		slog.Error("failed on investHelperService.AddStockToPortfolio", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return c.Send(internalErrMsg)
	}

	return c.Edit(telebotConverter.StockDetailResponse(stock, nil))
}

func (ctrl *Controller) ProcessDeleteStock(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.ProcessDeleteStock"
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		return c.Send(internalErrMsg)
	}

	if chatSession.StockTicker == "" {
		slog.Error("stockTicker is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return c.Send(internalErrMsg)
	}

	if chatSession.PortfolioID == 0 {
		slog.Error("PortfolioID is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return c.Send(internalErrMsg)
	}

	portfolioPage, err := ctrl.investHelperService.DeleteStockFromPortfolio(ctx, chatSession.PortfolioID, chatSession.StockTicker)
	if err != nil {
		slog.Error("failed on investHelperService.DeleteStockFromPortfolio", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return c.Send(internalErrMsg)
	}

	return c.Edit(telebotConverter.PortfolioDetailsResponse(portfolioPage))
}

func (ctrl *Controller) ProcessBackToPortfolioFromStock(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.ProcessBackToPortfolioFromStock"
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		return c.Send(internalErrMsg)
	}

	if chatSession.PortfolioID == 0 {
		slog.Error("PortfolioID is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return c.Send(internalErrMsg)
	}

	portfolioPage, err := ctrl.investHelperService.GetPortfolioPage(ctx, chatSession.PortfolioID, 1)
	if err != nil {
		slog.Error("failed on investHelperService.GetPortfolioPage", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return c.Send(internalErrMsg)
	}

	chatSession.Action = model.DefaultAction
	chatSession.StockChanges = nil
	chatSession.StockTicker = ""
	go ctrl.session.SetSession(context.WithoutCancel(ctx), strconv.FormatInt(c.Chat().ID, 10), chatSession)

	return c.Edit(telebotConverter.PortfolioDetailsResponse(portfolioPage))
}

func (ctrl *Controller) SaveStockChanges(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.SaveStockChanges"

	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		return c.Send(internalErrMsg)
	}

	if chatSession.StockChanges == nil { // просто отрисуем текущий stock без изменений
		stock, err := ctrl.investHelperService.GetPortfolioStockInfo(ctx, chatSession.StockTicker, chatSession.PortfolioID)
		if err != nil {
			slog.Error("failed on investHelperService.GetPortfolioStockInfo", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
			return c.Send(internalErrMsg)
		}
		return c.Send(telebotConverter.StockDetailResponse(stock, nil))
	}

	if chatSession.StockTicker == "" {
		slog.Error("stockTicker is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return c.Send(internalErrMsg)
	}

	if chatSession.PortfolioID == 0 {
		slog.Error("PortfolioID is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return c.Send(internalErrMsg)
	}

	stock, err := ctrl.investHelperService.SaveStockChangesToPortfolio(
		ctx,
		chatSession.PortfolioID,
		chatSession.StockTicker,
		chatSession.StockChanges.NewTargetWeight,
		chatSession.StockChanges.Quantity,
		chatSession.StockChanges.CustomPrice,
	)
	if err != nil {
		slog.Error("got error from investHelperService.SaveStockChangesToPortfolio", slog.String("rqID", rqID), slog.String("op", op))
		return c.Send(internalErrMsg)
	}

	chatSession.StockChanges = nil
	go ctrl.session.SetSession(ctx, strconv.FormatInt(c.Chat().ID, 10), chatSession)

	return c.Edit(telebotConverter.StockDetailResponse(stock, nil))
}
