package telegram

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/KotFed0t/invest_helper_bot/config"
	"github.com/KotFed0t/invest_helper_bot/data/session"
	"github.com/KotFed0t/invest_helper_bot/internal/converter/telebotConverter"
	"github.com/KotFed0t/invest_helper_bot/internal/model"
	"github.com/KotFed0t/invest_helper_bot/internal/model/moexModel"
	"github.com/KotFed0t/invest_helper_bot/internal/model/tg/tgCallback.go"
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
	DeleteStockFromPortfolio(ctx context.Context, portfolioID int64, ticker string) error
	GetPortfolioPage(ctx context.Context, portfolioID int64, page int) (model.PortfolioPage, error)
	CalculatePurchase(ctx context.Context, portfolioID int64, purchaseSum decimal.Decimal) ([]model.StockPurchase, error)
	GetPortfolios(ctx context.Context, chatID int64, page int) (portfolios []model.Portfolio, hasNextPage bool, err error)
	RebalanceWeights(ctx context.Context, portfolioID int64) error
	DeletePortfolio(ctx context.Context, portfolioID int64) error
	GeneratePortfoliosReport(ctx context.Context, chatID int64) (fileBytes []byte, filename string, err error)
	UploadFileToCloud(ctx context.Context, reader io.Reader, filename string) (downloadLink string, err error)
	ApplyCalculatedPurchaseToPortfolio(ctx context.Context, portfolioID int64, stocksToPurchase []model.StockPurchase) error
}

type Session interface {
	GetSession(ctx context.Context, key string) (model.Session, error)
	SetSession(ctx context.Context, key string, session model.Session) error
}

type Controller struct {
	cfg                 *config.Config
	investHelperService InvestHelperService
	session             Session
}

func NewController(cfg *config.Config, investHelperService InvestHelperService, session Session) *Controller {
	return &Controller{
		cfg:                 cfg,
		investHelperService: investHelperService,
		session:             session,
	}
}

func (ctrl *Controller) Start(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	err := ctrl.investHelperService.RegUser(context.WithoutCancel(ctx), c.Chat().ID)
	if err != nil {
		return c.Send("Регистрация завершилась с ошибкой. Вызовите команду /start еще раз.")
	}
	return c.Reply("Добро пожаловать! Можешь начать выбрав одну из команд в меню.")
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

	chatSession, _ := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	defer func() {
		chatSession.Action = model.DefaultAction
		go ctrl.session.SetSession(ctx, strconv.FormatInt(c.Chat().ID, 10), chatSession)
	}()

	portfolioID, err := ctrl.investHelperService.CreateStocksPortfolio(ctx, c.Message().Text, c.Chat().ID)
	if err != nil {
		slog.Error("got error from investHelperService.CreateStocksPortfolio", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	chatSession.PortfolioID = portfolioID

	portfolio := model.PortfolioPage{
		PortfolioSummary: model.PortfolioSummary{
			Portfolio: model.Portfolio{
				PortfolioName: c.Message().Text,
			},
		},
	}
	return c.Send(telebotConverter.PortfolioDetailsResponse(portfolio, ctrl.cfg.StocksPerPage))
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
		if errors.Is(err, session.ErrNotFound) {
			return ctrl.ProcessBackToPortfolioList(c)
		}
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	chatSession.Action = model.ExpectingTicker
	err = ctrl.session.SetSession(ctx, strconv.FormatInt(c.Chat().ID, 10), chatSession)
	if err != nil {
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	return c.Edit("Введите тикер")
}

func (ctrl *Controller) ProcessAddStock(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)

	chatSession, _ := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	defer func() {
		chatSession.Action = model.DefaultAction
		go ctrl.session.SetSession(ctx, strconv.FormatInt(c.Chat().ID, 10), chatSession)
	}()

	ticker := strings.ToUpper(c.Message().Text)

	stockInfo, err := ctrl.investHelperService.GetStockInfo(ctx, ticker)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return c.Send("Не удалось найти указанный тикер", telebotConverter.StockNotFoundMarkup())
		}
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	chatSession.StockTicker = ticker

	return c.Send(telebotConverter.StockAddResponse(stockInfo))
}

func (ctrl *Controller) InitChangeWeight(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return ctrl.ProcessBackToPortfolioList(c)
		}
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	chatSession.Action = model.ExpectingWeight
	err = ctrl.session.SetSession(ctx, strconv.FormatInt(c.Chat().ID, 10), chatSession)
	if err != nil {
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	return c.Edit("введите новое значение веса:")
}

func (ctrl *Controller) ProcessChangeWeight(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.ProcessChangeWeight"
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return ctrl.ProcessBackToPortfolioList(c)
		}
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
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
		return ctrl.ProcessBackToPortfolioList(c)
	}

	if chatSession.PortfolioID == 0 {
		slog.Error("PortfolioID is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return ctrl.ProcessBackToPortfolioList(c)
	}

	stock, err := ctrl.investHelperService.GetPortfolioStockInfo(ctx, chatSession.StockTicker, chatSession.PortfolioID)
	if err != nil && !errors.Is(err, service.ErrActualStockInfoUnavailable) {
		slog.Error("failed on investHelperService.GetPortfolioStockInfo", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
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
		if errors.Is(err, session.ErrNotFound) {
			return ctrl.ProcessBackToPortfolioList(c)
		}
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	chatSession.Action = model.ExpectingBuyStockQuantity
	err = ctrl.session.SetSession(ctx, strconv.FormatInt(c.Chat().ID, 10), chatSession)
	if err != nil {
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	return c.Edit("введите кол-во акций к покупке:")
}

func (ctrl *Controller) ProcessBuyStock(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.ProcessBuyStock"
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return ctrl.ProcessBackToPortfolioList(c)
		}
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
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
		return ctrl.ProcessBackToPortfolioList(c)
	}

	if chatSession.PortfolioID == 0 {
		slog.Error("PortfolioID is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return ctrl.ProcessBackToPortfolioList(c)
	}

	stock, err := ctrl.investHelperService.GetPortfolioStockInfo(ctx, chatSession.StockTicker, chatSession.PortfolioID)
	if err != nil && !errors.Is(err, service.ErrActualStockInfoUnavailable) {
		slog.Error("failed on investHelperService.GetPortfolioStockInfo", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
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
		if errors.Is(err, session.ErrNotFound) {
			return ctrl.ProcessBackToPortfolioList(c)
		}
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	chatSession.Action = model.ExpectingSellStockQuantity
	err = ctrl.session.SetSession(ctx, strconv.FormatInt(c.Chat().ID, 10), chatSession)
	if err != nil {
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	return c.Edit("введите кол-во акций к продаже:")
}

func (ctrl *Controller) ProcessSellStock(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.ProcessSellStock"
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return ctrl.ProcessBackToPortfolioList(c)
		}
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	quantity, err := strconv.Atoi(c.Message().Text)
	if err != nil || quantity <= 0 {
		return c.Send("количество должно быть целым числом больше 0, введите корректное значение:")
	}

	if chatSession.StockTicker == "" {
		slog.Error("stockTicker is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return ctrl.ProcessBackToPortfolioList(c)
	}

	if chatSession.PortfolioID == 0 {
		slog.Error("PortfolioID is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return ctrl.ProcessBackToPortfolioList(c)
	}

	stock, err := ctrl.investHelperService.GetPortfolioStockInfo(ctx, chatSession.StockTicker, chatSession.PortfolioID)
	if err != nil && !errors.Is(err, service.ErrActualStockInfoUnavailable) {
		slog.Error("failed on investHelperService.GetPortfolioStockInfo", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
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
		if errors.Is(err, session.ErrNotFound) {
			return ctrl.ProcessBackToPortfolioList(c)
		}
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	chatSession.Action = model.ExpectingChangePrice
	err = ctrl.session.SetSession(ctx, strconv.FormatInt(c.Chat().ID, 10), chatSession)
	if err != nil {
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	return c.Edit("введите цену за 1 акцию:")
}

func (ctrl *Controller) ProcessChangePrice(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.ProcessChangePrice"
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return ctrl.ProcessBackToPortfolioList(c)
		}
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	input := strings.Replace(c.Message().Text, ",", ".", 1)

	price, err := decimal.NewFromString(input)
	if err != nil || price.IsNegative() || price.IsZero() {
		return c.Send("цена должна быть числом больше 0, введите корректное значение:")
	}

	if chatSession.StockTicker == "" {
		slog.Error("stockTicker is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return ctrl.ProcessBackToPortfolioList(c)
	}

	if chatSession.PortfolioID == 0 {
		slog.Error("PortfolioID is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return ctrl.ProcessBackToPortfolioList(c)
	}

	stock, err := ctrl.investHelperService.GetPortfolioStockInfo(ctx, chatSession.StockTicker, chatSession.PortfolioID)
	if err != nil && !errors.Is(err, service.ErrActualStockInfoUnavailable) {
		slog.Error("failed on investHelperService.GetPortfolioStockInfo", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
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
		if errors.Is(err, session.ErrNotFound) {
			return ctrl.ProcessBackToPortfolioList(c)
		}
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	if chatSession.StockTicker == "" {
		slog.Error("stockTicker is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return ctrl.ProcessBackToPortfolioList(c)
	}

	if chatSession.PortfolioID == 0 {
		slog.Error("PortfolioID is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return ctrl.ProcessBackToPortfolioList(c)
	}

	stock, err := ctrl.investHelperService.AddStockToPortfolio(ctx, chatSession.StockTicker, chatSession.PortfolioID, c.Chat().ID)
	if err != nil && !errors.Is(err, service.ErrActualStockInfoUnavailable) {
		slog.Error("failed on investHelperService.AddStockToPortfolio", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	return c.Edit(telebotConverter.StockDetailResponse(stock, nil))
}

func (ctrl *Controller) ProcessDeleteStock(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.ProcessDeleteStock"
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return ctrl.ProcessBackToPortfolioList(c)
		}
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	if chatSession.StockTicker == "" {
		slog.Error("stockTicker is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return ctrl.ProcessBackToPortfolioList(c)
	}

	if chatSession.PortfolioID == 0 {
		slog.Error("PortfolioID is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return ctrl.ProcessBackToPortfolioList(c)
	}

	page := 1
	if chatSession.CurPortfolioDetailsPage > 0 {
		page = chatSession.CurPortfolioDetailsPage
	}

	err = ctrl.investHelperService.DeleteStockFromPortfolio(ctx, chatSession.PortfolioID, chatSession.StockTicker)
	if err != nil {
		slog.Error("failed on investHelperService.DeleteStockFromPortfolio", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	go ctrl.sendAutoDeleteMsg(c, "акция успешно удалена")

	portfolioPage, err := ctrl.investHelperService.GetPortfolioPage(ctx, chatSession.PortfolioID, page)
	if err != nil {
		slog.Error("failed on investHelperService.GetPortfolioPage", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	return c.Edit(telebotConverter.PortfolioDetailsResponse(portfolioPage, ctrl.cfg.StocksPerPage))
}

func (ctrl *Controller) ProcessBackToPortfolio(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.ProcessBackToPortfolio"
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return ctrl.ProcessBackToPortfolioList(c)
		}
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	if chatSession.PortfolioID == 0 {
		slog.Error("PortfolioID is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return ctrl.ProcessBackToPortfolioList(c)
	}

	page := 1
	if chatSession.CurPortfolioDetailsPage != 0 {
		page = chatSession.CurPortfolioDetailsPage
	}

	portfolioPage, err := ctrl.investHelperService.GetPortfolioPage(ctx, chatSession.PortfolioID, page)
	if err != nil {
		slog.Error("failed on investHelperService.GetPortfolioPage", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	chatSession.Action = model.DefaultAction
	chatSession.StockChanges = nil
	chatSession.StockTicker = ""
	chatSession.StocksToPurchase = nil
	go ctrl.session.SetSession(context.WithoutCancel(ctx), strconv.FormatInt(c.Chat().ID, 10), chatSession)

	return c.Edit(telebotConverter.PortfolioDetailsResponse(portfolioPage, ctrl.cfg.StocksPerPage))
}

func (ctrl *Controller) SaveStockChanges(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.SaveStockChanges"

	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return ctrl.ProcessBackToPortfolioList(c)
		}
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	if chatSession.StockChanges == nil { // просто отрисуем текущий stock без изменений
		stock, err := ctrl.investHelperService.GetPortfolioStockInfo(ctx, chatSession.StockTicker, chatSession.PortfolioID)
		if err != nil && !errors.Is(err, service.ErrActualStockInfoUnavailable) {
			slog.Error("failed on investHelperService.GetPortfolioStockInfo", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
			return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
		}
		return c.Send(telebotConverter.StockDetailResponse(stock, nil))
	}

	if chatSession.StockTicker == "" {
		slog.Error("stockTicker is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return ctrl.ProcessBackToPortfolioList(c)
	}

	if chatSession.PortfolioID == 0 {
		slog.Error("PortfolioID is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return ctrl.ProcessBackToPortfolioList(c)
	}

	stock, err := ctrl.investHelperService.SaveStockChangesToPortfolio(
		ctx,
		chatSession.PortfolioID,
		chatSession.StockTicker,
		chatSession.StockChanges.NewTargetWeight,
		chatSession.StockChanges.Quantity,
		chatSession.StockChanges.CustomPrice,
	)
	if err != nil && !errors.Is(err, service.ErrActualStockInfoUnavailable) {
		slog.Error("got error from investHelperService.SaveStockChangesToPortfolio", slog.String("rqID", rqID), slog.String("op", op))
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	chatSession.StockChanges = nil
	go ctrl.session.SetSession(ctx, strconv.FormatInt(c.Chat().ID, 10), chatSession)

	return c.Edit(telebotConverter.StockDetailResponse(stock, nil))
}

func (ctrl *Controller) GoToEditStock(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.GoToEditStock"
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return ctrl.ProcessBackToPortfolioList(c)
		}
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	if chatSession.PortfolioID == 0 {
		slog.Error("PortfolioID is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return ctrl.ProcessBackToPortfolioList(c)
	}

	ticker := strings.TrimPrefix(c.Callback().Data, fmt.Sprintf("\f%s", tgCallback.EditStockPrefix))

	stockInfo, err := ctrl.investHelperService.GetPortfolioStockInfo(ctx, ticker, chatSession.PortfolioID)
	if err != nil && !errors.Is(err, service.ErrActualStockInfoUnavailable) {
		slog.Error("failed on investHelperService.GetPortfolioStockInfo", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	chatSession.StockTicker = ticker
	go ctrl.session.SetSession(context.WithoutCancel(ctx), strconv.FormatInt(c.Chat().ID, 10), chatSession)

	return c.Edit(telebotConverter.StockDetailResponse(stockInfo, nil))
}

func (ctrl *Controller) GoToPortfolioPage(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.NextPagePortfolio"
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return ctrl.ProcessBackToPortfolioList(c)
		}
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	if chatSession.PortfolioID == 0 {
		slog.Error("PortfolioID is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return ctrl.ProcessBackToPortfolioList(c)
	}

	pageStr := strings.TrimPrefix(c.Callback().Data, fmt.Sprintf("\f%s", tgCallback.ToPortfolioPage))
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		slog.Error("can't convert pageStr to int", slog.String("rqID", rqID), slog.String("op", op), slog.String("pageStr", pageStr))
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	portfolioPage, err := ctrl.investHelperService.GetPortfolioPage(ctx, chatSession.PortfolioID, page)
	if err != nil {
		slog.Error("failed on investHelperService.GetPortfolioPage", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	chatSession.CurPortfolioDetailsPage = page
	go ctrl.session.SetSession(context.WithoutCancel(ctx), strconv.FormatInt(c.Chat().ID, 10), chatSession)

	return c.Edit(telebotConverter.PortfolioDetailsResponse(portfolioPage, ctrl.cfg.StocksPerPage))
}

func (ctrl *Controller) InitCalculatePurchase(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return ctrl.ProcessBackToPortfolioList(c)
		}
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	chatSession.Action = model.ExpectingPurchaseSum
	err = ctrl.session.SetSession(ctx, strconv.FormatInt(c.Chat().ID, 10), chatSession)
	if err != nil {
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	return c.Edit("введите сумму закупки:")
}

func (ctrl *Controller) ProcessCalculatePurchase(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.ProcessCalculatePurchase"
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return ctrl.ProcessBackToPortfolioList(c)
		}
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	if chatSession.PortfolioID == 0 {
		slog.Error("PortfolioID is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return ctrl.ProcessBackToPortfolioList(c)
	}

	input := strings.Replace(c.Message().Text, ",", ".", 1)

	purchaseSum, err := decimal.NewFromString(input)
	if err != nil || purchaseSum.IsNegative() || purchaseSum.IsZero() {
		return c.Send("Сумма должна быть положительным числом > 0, введите корректное значение:")
	}

	stocksToPurchase, err := ctrl.investHelperService.CalculatePurchase(ctx, chatSession.PortfolioID, purchaseSum)
	if err != nil {
		slog.Error("failed on investHelperService.CalculatePurchase", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	if len(stocksToPurchase) == 0 {
		return c.Send("нельзя купить соответствуя индексу на указанную сумму, введите сумму больше:")
	}

	chatSession.Action = model.DefaultAction
	chatSession.StocksToPurchase = stocksToPurchase
	go ctrl.session.SetSession(ctx, strconv.FormatInt(c.Chat().ID, 10), chatSession)

	texts, markup := telebotConverter.CalculatedStockPurchaseResponse(stocksToPurchase, purchaseSum)
	for _, text := range texts {
		_ = c.Send(text)
	}

	return c.Send("навигация:", markup)
}

func (ctrl *Controller) GetPortfolios(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.GetPortfolios"
	var err error

	page := 1
	if c.Callback() != nil {
		pageStr := strings.TrimPrefix(c.Callback().Data, fmt.Sprintf("\f%s", tgCallback.ToPortfolioListPage))
		page, err = strconv.Atoi(pageStr)
		if err != nil {
			page = 1
		}
	}

	portfolios, hasNextPage, err := ctrl.investHelperService.GetPortfolios(ctx, c.Chat().ID, page)
	if err != nil {
		slog.Error("failed on investHelperService.GetPortfolios", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	chatSession, _ := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	chatSession.CurPortfolioListPage = page
	go ctrl.session.SetSession(context.WithoutCancel(ctx), strconv.FormatInt(c.Chat().ID, 10), chatSession)

	// при пагинации нужен Edit
	if c.Callback() != nil {
		return c.Edit(telebotConverter.PortfolioListResponse(portfolios, ctrl.cfg.PortfoliosPerPage, page, hasNextPage))
	}
	return c.Send(telebotConverter.PortfolioListResponse(portfolios, ctrl.cfg.PortfoliosPerPage, page, hasNextPage))
}

func (ctrl *Controller) GoToEditPortfolio(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.GoToEditPortfolio"

	callbackStr := strings.TrimPrefix(c.Callback().Data, fmt.Sprintf("\f%s", tgCallback.EditPortfolioPrefix))
	portfolioID, err := strconv.ParseInt(callbackStr, 10, 64)
	if err != nil {
		slog.Error("invalid portfolioID in callback", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()), slog.String("callback", c.Callback().Data))
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	portfolioPage, err := ctrl.investHelperService.GetPortfolioPage(ctx, portfolioID, 1)
	if err != nil {
		slog.Error("failed on investHelperService.GetPortfolioPage", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	chatSession, _ := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	chatSession.PortfolioID = portfolioID
	go ctrl.session.SetSession(context.WithoutCancel(ctx), strconv.FormatInt(c.Chat().ID, 10), chatSession)

	return c.Edit(telebotConverter.PortfolioDetailsResponse(portfolioPage, ctrl.cfg.StocksPerPage))
}

func (ctrl *Controller) ProcessBackToPortfolioList(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.ProcessBackToPortfolioList"
	chatSession, _ := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)

	page := 1
	if chatSession.CurPortfolioListPage != 0 {
		page = chatSession.CurPortfolioListPage
	}

	portfolios, hasNextPage, err := ctrl.investHelperService.GetPortfolios(ctx, c.Chat().ID, page)
	if err != nil {
		slog.Error("failed on investHelperService.GetPortfolios", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	// обнуляем все в сессии, кроме страницы pageList
	chatSession = model.Session{CurPortfolioListPage: page}
	go ctrl.session.SetSession(context.WithoutCancel(ctx), strconv.FormatInt(c.Chat().ID, 10), chatSession)

	return c.Edit(telebotConverter.PortfolioListResponse(portfolios, ctrl.cfg.PortfoliosPerPage, page, hasNextPage))
}

func (ctrl *Controller) RebalanceWeights(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.RebalanceWeights"
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return ctrl.ProcessBackToPortfolioList(c)
		}
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	if chatSession.PortfolioID == 0 {
		slog.Error("PortfolioID is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return ctrl.ProcessBackToPortfolioList(c)
	}

	err = ctrl.investHelperService.RebalanceWeights(ctx, chatSession.PortfolioID)
	if err != nil {
		slog.Error("failed on investHelperService.RebalanceWeights", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	page := 1
	if chatSession.CurPortfolioDetailsPage != 0 {
		page = chatSession.CurPortfolioDetailsPage
	}

	portfolioPage, err := ctrl.investHelperService.GetPortfolioPage(ctx, chatSession.PortfolioID, page)
	if err != nil {
		slog.Error("failed on investHelperService.GetPortfolioPage", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	go ctrl.sendAutoDeleteMsg(c, "ребаланс произведен успешно")

	return c.Edit(telebotConverter.PortfolioDetailsResponse(portfolioPage, ctrl.cfg.StocksPerPage))
}

func (ctrl *Controller) sendAutoDeleteMsg(c tele.Context, text string) error {
	msg, err := c.Bot().Send(c.Chat(), text)
	if err != nil {
		return err
	}

	time.AfterFunc(5*time.Second, func() {
		c.Bot().Delete(msg)
	})
	return nil
}

func (ctrl *Controller) InitDeletePortfolio(c tele.Context) error {
	return c.Edit(
		"Подтвердите удаление портфеля. Это действие необратимо, будут удалены все данные по портфелю и история операций!",
		telebotConverter.DeletePortfolioConfirmation(),
	)
}

func (ctrl *Controller) ProcessDeletePortfolio(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.ProcessDeletePortfolio"
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return ctrl.ProcessBackToPortfolioList(c)
		}
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	if chatSession.PortfolioID == 0 {
		slog.Error("PortfolioID is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return ctrl.ProcessBackToPortfolioList(c)
	}

	err = ctrl.investHelperService.DeletePortfolio(ctx, chatSession.PortfolioID)
	if err != nil {
		slog.Error("failed on investHelperService.DeletePortfolio", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	go ctrl.sendAutoDeleteMsg(c, "портфель успешно удален")

	portfolios, hasNextPage, err := ctrl.investHelperService.GetPortfolios(ctx, c.Chat().ID, 1)
	if err != nil {
		slog.Error("failed on investHelperService.GetPortfolios", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	return c.Edit(telebotConverter.PortfolioListResponse(portfolios, ctrl.cfg.PortfoliosPerPage, 1, hasNextPage))
}

func (ctrl *Controller) ApplyCalculatedPurchaseToPortfolio(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.ApplyCalculatedPurchaseToPortfolio"
	chatSession, err := ctrl.getSessionFromTeleCtxOrStorage(ctx, c)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return ctrl.ProcessBackToPortfolioList(c)
		}
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	if chatSession.PortfolioID == 0 {
		slog.Error("PortfolioID is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return ctrl.ProcessBackToPortfolioList(c)
	}

	if len(chatSession.StocksToPurchase) == 0 {
		slog.Error("StocksToPurchase is empty in chatSession", slog.String("rqID", rqID), slog.String("op", op))
		return ctrl.ProcessBackToPortfolioList(c)
	}

	err = ctrl.investHelperService.ApplyCalculatedPurchaseToPortfolio(ctx, chatSession.PortfolioID, chatSession.StocksToPurchase)
	if err != nil {
		slog.Error("failed on investHelperService.ApplyCalculatedPurchaseToPortfolio", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	go ctrl.sendAutoDeleteMsg(c, "операции успешно применены")

	_, markup := telebotConverter.CalculatedStockPurchaseResponse(nil, decimal.NewFromInt(0))

	return c.Edit("навигация:", markup)
}

func (ctrl *Controller) GenerateReport(c tele.Context) error {
	ctx := utils.CreateCtxWithRqID(c)
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Controller.GenerateReport"

	fileBytes, filename, err := ctrl.investHelperService.GeneratePortfoliosReport(ctx, c.Chat().ID)
	if err != nil {
		slog.Error("failed on investHelperService.GeneratePortfoliosReport", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	if len(fileBytes) < ctrl.cfg.Telegram.FileLimitInBytes {
		doc := &tele.Document{
			File:     tele.File{FileReader: bytes.NewReader(fileBytes)},
			FileName: filename,
		}
		return c.Send(doc)
	}

	// иначе загружаем в облако и отправляем ссылку на скачивание
	downloadLink, err := ctrl.investHelperService.UploadFileToCloud(ctx, bytes.NewReader(fileBytes), filename)
	if err != nil {
		slog.Error("failed on investHelperService.UploadFileToCloud", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return ctrl.sendAutoDeleteMsg(c, internalErrMsg)
	}

	return c.Send(downloadLink)
}

// TODO сделать job для удаления старых файлов на google drive

// TODO поправить логирование излишнее

// TODO юнит тесты сервисного слоя

// TODO напоминалки?
