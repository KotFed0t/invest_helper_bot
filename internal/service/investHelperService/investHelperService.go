package investHelperService

import (
	"context"
	"errors"
	"log/slog"

	"github.com/KotFed0t/invest_helper_bot/data/repository"
	"github.com/KotFed0t/invest_helper_bot/internal/externalApi"
	"github.com/KotFed0t/invest_helper_bot/internal/model"
	"github.com/KotFed0t/invest_helper_bot/internal/model/dbModel"
	"github.com/KotFed0t/invest_helper_bot/internal/model/moexModel"
	"github.com/KotFed0t/invest_helper_bot/internal/service"
	"github.com/KotFed0t/invest_helper_bot/utils"
	"github.com/shopspring/decimal"
)

type MoexApi interface {
	GetStocInfo(ctx context.Context, ticker string) (moexModel.StockInfo, error)
	GetStocsInfo(ctx context.Context, tickers []string) (map[string]moexModel.StockInfo, error)
}

type Cache interface {
	GetStockInfo(ctx context.Context, ticker string) (moexModel.StockInfo, error)
	GetStocksInfo(ctx context.Context, tickers []string) (map[string]moexModel.StockInfo, error)
	GetPortfolioStock(ctx context.Context, ticker string, portfolioID int64) (model.Stock, error)
	GetPortfolioSummary(ctx context.Context, portfolioID int64) (model.PortfolioSummary, error)
	SetPortfolioStock(ctx context.Context, portfolioID int64, stock model.Stock) error
	SetPortfolioSummary(ctx context.Context, portfolioID int64, summary model.PortfolioSummary) error
	FlushPortfolioCache(ctx context.Context, portfolioID int64) error
}

type Repository interface {
	RegUser(ctx context.Context, chatID int64) (userID int64, err error)
	CreateStocksPortfolio(ctx context.Context, name string, userID int64) (portfolioID int64, err error)
	GetUserID(ctx context.Context, chatID int64) (userID int64, err error)
	GetStockFromPortfolio(ctx context.Context, ticker string, portfolioID int64) (stock dbModel.Stock, err error)
	GetStocksFromPortfolio(ctx context.Context, portfolioID int64) (stocks []dbModel.Stock, err error)
	InsertStockToPortfolio(ctx context.Context, portfolioID int64, ticker string, userID int64) (err error)
	UpdatePortfolioStock(ctx context.Context, portfolioID int64, ticker string, weight *decimal.Decimal, quantity *int) (err error)
	InsertStockOperationToHistory(ctx context.Context, portfolioID int64, stockOperation model.StockOperation) (err error)
}

type InvestHelperService struct {
	repo    Repository
	cache   Cache
	moexApi MoexApi
}

func New(repo Repository, cache Cache, moexApi MoexApi) *InvestHelperService {
	return &InvestHelperService{
		repo:    repo,
		cache:   cache,
		moexApi: moexApi,
	}
}

func (s *InvestHelperService) RegUser(ctx context.Context, chatID int64) error {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "InvestHelperService.RegUser"

	slog.Debug("RegUser start", slog.String("rqID", rqID), slog.String("op", op), slog.Int64("chatID", chatID))
	defer func() {
		slog.Debug("RegUser finished", slog.String("rqID", rqID), slog.String("op", op), slog.Int64("chatID", chatID))
	}()

	_, err := s.repo.RegUser(ctx, chatID)
	if err != nil {
		slog.Error("got error from repo.RegUser", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return err
	}

	return nil
}

func (s *InvestHelperService) CreateStocksPortfolio(ctx context.Context, portfolioName string, chatID int64) (portfolioID int64, err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "InvestHelperService.CreateStocksPortfolio"

	slog.Debug("CreateStocksPortfolio start", slog.String("rqID", rqID), slog.String("op", op), slog.String("portfolioName", portfolioName), slog.Int64("chatID", chatID))
	defer func() {
		slog.Debug("CreateStocksPortfolio finished", slog.String("rqID", rqID), slog.String("op", op), slog.String("portfolioName", portfolioName), slog.Int64("chatID", chatID))
	}()

	userID, err := s.repo.GetUserID(ctx, chatID)
	if err != nil {
		slog.Error("got error from repo.GetUserID", slog.String("rqID", rqID), slog.String("err", err.Error()))
		return 0, err
	}

	portfolioID, err = s.repo.CreateStocksPortfolio(ctx, portfolioName, userID)
	if err != nil {
		slog.Error("got error from repo.CreateStocksPortfolio", slog.String("rqID", rqID), slog.String("err", err.Error()))
		return 0, err
	}

	return portfolioID, nil
}

func (s *InvestHelperService) GetPortfolioInfo(ctx context.Context, portfolioID, page int) {
	// выбираем все акции из портфеля
	// считаем totalBalance
	// getTotalBalance(pID) decimal - либо из кэша, либо считает полностью
	// еще total weight надо

	// выбираем акции для страницы по limit + 1 offset - офсет считаем как (page - 1) * limit
	// чтобы знать есть ли next

	// для выбранных акций посчитать cur weight и стоимость
}

func (s *InvestHelperService) GetStockInfo(ctx context.Context, ticker string) (stockInfo moexModel.StockInfo, err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "InvestHelperService.GetStockInfo"

	slog.Debug("GetStockInfo start", slog.String("rqID", rqID), slog.String("op", op), slog.String("ticker", ticker))
	defer func() {
		slog.Debug("GetStockInfo finished", slog.String("rqID", rqID), slog.String("op", op), slog.String("ticker", ticker))
	}()

	stockInfo, err = s.cache.GetStockInfo(ctx, ticker)
	if err != nil {
		slog.Warn("can't get stock info from cache", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))

		stockInfo, err = s.moexApi.GetStocInfo(ctx, ticker)
		if err != nil {
			if errors.Is(err, externalApi.ErrNotFound) {
				slog.Warn("stock not found in moexApi", slog.String("rqID", rqID), slog.String("op", op))
				return moexModel.StockInfo{}, service.ErrNotFound
			}
			slog.Error("can't get stock info from moexApi", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
			return moexModel.StockInfo{}, err
		}
	}

	if stockInfo.Status == false || stockInfo.Price.IsZero() {
		return moexModel.StockInfo{}, service.ErrStockNotActive
	}

	return stockInfo, nil
}

func (s *InvestHelperService) addStockToPortfolio(ctx context.Context, ticker string, portfolioID, chatID int64) error {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "InvestHelperService.addStockToPortfolio"

	userID, err := s.repo.GetUserID(ctx, chatID)
	if err != nil {
		return err
	}

	err = s.repo.InsertStockToPortfolio(ctx, portfolioID, ticker, userID)
	if err != nil {
		if errors.Is(err, repository.ErrAlreadyExists) {
			return nil
		}
		slog.Error("got error from repo.InsertStockToPortfolio", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return err
	}

	// TODO сбросить кэш, если реально добавилась акция (кажется только кэш страниц будет достаточно)

	return nil
}

func (s *InvestHelperService) AddStockToPortfolio(ctx context.Context, ticker string, portfolioID, chatID int64) (model.Stock, error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "InvestHelperService.AddStockToPortfolio"

	slog.Debug("AddStockToPortfolio start", slog.String("rqID", rqID), slog.String("op", op), slog.String("ticker", ticker))
	defer func() {
		slog.Debug("AddStockToPortfolio finished", slog.String("rqID", rqID), slog.String("op", op), slog.String("ticker", ticker))
	}()

	err := s.addStockToPortfolio(ctx, ticker, portfolioID, chatID)
	if err != nil {
		return model.Stock{}, err
	}

	return s.GetPortfolioStockInfo(ctx, ticker, portfolioID)
}

func (s *InvestHelperService) GetPortfolioSummaryInfo(ctx context.Context, portfolioID int64) (summary model.PortfolioSummary, err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "InvestHelperService.GetPortfolioSummaryInfo"

	slog.Debug("GetPortfolioSummaryInfo start", slog.String("rqID", rqID), slog.String("op", op), slog.Int64("portfolioID", portfolioID))
	defer func() {
		slog.Debug("GetPortfolioSummaryInfo finished", slog.String("rqID", rqID), slog.String("op", op), slog.Int64("portfolioID", portfolioID))
	}()

	summary, err = s.cache.GetPortfolioSummary(ctx, portfolioID)
	if err == nil {
		return summary, nil
	}

	slog.Warn("can't get portfolio summary info from cache", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))

	// селектим все акции из БД
	stocks, err := s.repo.GetStocksFromPortfolio(ctx, portfolioID)
	if err != nil {
		return model.PortfolioSummary{}, err
	}

	slog.Debug("got stocks from DB", slog.String("rqID", rqID), slog.String("op", op), slog.Any("stocks", stocks))

	// получаем актуальные цены для акций
	tickers := make([]string, 0, len(stocks))
	for _, stock := range stocks {
		tickers = append(tickers, stock.Ticker)
	}

	stocksInfoMap, err := s.cache.GetStocksInfo(ctx, tickers)
	if err != nil {
		slog.Warn("can't get stocks info from cache", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))

		stocksInfoMap, err = s.moexApi.GetStocsInfo(ctx, tickers)
		if err != nil {
			return model.PortfolioSummary{}, err
		}
	}

	slog.Debug("got stocksInfoMap", slog.String("rqID", rqID), slog.String("op", op), slog.Any("stocksInfoMap", stocksInfoMap))

	// считаем totalBalance и totalWeight
	for _, stock := range stocks {
		summary.TotalWeight = summary.TotalWeight.Add(stock.Weight)

		stockInfo, ok := stocksInfoMap[stock.Ticker]
		if !ok {
			slog.Error("ticker not found in stocksInfoMap", slog.String("rqID", rqID), slog.String("op", op), slog.String("ticker", stock.Ticker))
			return model.PortfolioSummary{}, errors.New("can't calculate summary cause got partial info")
		}

		if !stock.Weight.IsZero() {
			summary.TotalBalance = summary.TotalBalance.Add(stockInfo.Price.Mul(decimal.NewFromInt(int64(stock.Quantity))))
		} else {
			summary.BalanceOutsideIndex = summary.BalanceOutsideIndex.Add(stockInfo.Price.Mul(decimal.NewFromInt(int64(stock.Quantity))))
		}
	}

	// сохраняем в кэш
	go s.cache.SetPortfolioSummary(context.WithoutCancel(ctx), portfolioID, summary)

	return summary, nil
}

func (s *InvestHelperService) GetPortfolioStockInfo(ctx context.Context, ticker string, portfolioID int64) (model.Stock, error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "InvestHelperService.GetPortfolioStockInfo"

	slog.Debug("GetStockInfoFromPortfolio start", slog.String("rqID", rqID), slog.String("op", op), slog.String("ticker", ticker))
	defer func() {
		slog.Debug("GetStockInfoFromPortfolio finished", slog.String("rqID", rqID), slog.String("op", op), slog.String("ticker", ticker))
	}()

	// полностью из кэша
	stock, err := s.cache.GetPortfolioStock(ctx, ticker, portfolioID)
	if err == nil {
		slog.Info("got portfolio Stock info from cache", slog.String("rqID", rqID), slog.String("op", op), slog.Any("stockInfo", stock))
		return stock, nil
	}

	slog.Warn("can't get portfolioStock info from cache", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
	// считаем баланс
	portfolioSummary, err := s.GetPortfolioSummaryInfo(ctx, portfolioID)
	if err != nil {
		return model.Stock{}, err
	}

	// берем из БД
	stockDB, err := s.repo.GetStockFromPortfolio(ctx, ticker, portfolioID)
	if err != nil {
		return model.Stock{}, err
	}

	// обогащаем инфой (кэш или запрос в moex)
	stockInfo, err := s.GetStockInfo(ctx, ticker)
	if err != nil {
		// TODO может быть что акция перестала быть активной или цена пустая стала
		// в таком случае не ошибку вернуть, а позволить клиенту удалить ее из портфеля
		return model.Stock{}, err
	}

	stock = model.Stock{
		Ticker:       stockInfo.Ticker,
		Shortname:    stockInfo.Shortname,
		Lotsize:      stockInfo.Lotsize,
		TargetWeight: stockDB.Weight,
		Quantity:     stockDB.Quantity,
		Price:        stockInfo.Price,
		TotalPrice:   stockInfo.Price.Mul(decimal.NewFromInt(int64(stockDB.Quantity))),
	}

	if !portfolioSummary.TotalBalance.IsZero() {
		stock.ActualWeight = stock.TotalPrice.Div(portfolioSummary.TotalBalance).Mul(decimal.NewFromInt(100))
	}

	// в конце сохранить в кэш
	go s.cache.SetPortfolioStock(context.WithoutCancel(ctx), portfolioID, stock)

	return stock, nil
}

func (s *InvestHelperService) saveStockChangesToPortfolio(ctx context.Context, portfolioID int64, ticker string, weight *decimal.Decimal, quantity *int) error {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "InvestHelperService.saveStockChangesToPortfolio"

	err := s.repo.UpdatePortfolioStock(ctx, portfolioID, ticker, weight, quantity)
	if err != nil {
		slog.Error("got error from repo.UpdatePortfolioStock", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return err
	}

	err = s.cache.FlushPortfolioCache(ctx, portfolioID) // вызываем синхронно, так как конкурентно может не успеть удалиться и получим старую инфу
	if err != nil {
		slog.Error("got error from cache.FlushPortfolioCache", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
	}

	return nil
}

func (s *InvestHelperService) SaveStockChangesToPortfolio(
	ctx context.Context,
	portfolioID int64,
	ticker string,
	weight *decimal.Decimal,
	quantity *int,
	price *decimal.Decimal,
) (model.Stock, error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "InvestHelperService.SaveStockChangesToPortfolio"

	slog.Debug("SaveStockChangesToPortfolio start", slog.String("rqID", rqID), slog.String("op", op), slog.String("ticker", ticker))
	defer func() {
		slog.Debug("SaveStockChangesToPortfolio finished", slog.String("rqID", rqID), slog.String("op", op), slog.String("ticker", ticker))
	}()

	err := s.saveStockChangesToPortfolio(ctx, portfolioID, ticker, weight, quantity)
	if err != nil {
		return model.Stock{}, err
	}

	if quantity != nil { // если была покупка/продажа сохраняем операцию в историю
		go s.saveStockOperationToHistory(context.WithoutCancel(ctx), portfolioID, ticker, *quantity, price)
	}

	stock, err := s.GetPortfolioStockInfo(ctx, ticker, portfolioID)
	if err != nil {
		return model.Stock{}, err
	}

	return stock, nil
}

func (s *InvestHelperService) saveStockOperationToHistory(ctx context.Context, portfolioID int64, ticker string, quantity int, price *decimal.Decimal) error {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "InvestHelperService.saveStockOperationToHistory"

	slog.Debug("saveStockOperationToHistory start", slog.String("rqID", rqID), slog.String("op", op), slog.String("ticker", ticker))
	defer func() {
		slog.Debug("saveStockOperationToHistory finished", slog.String("rqID", rqID), slog.String("op", op), slog.String("ticker", ticker))
	}()

	stockInfo, err := s.GetStockInfo(ctx, ticker)
	if err != nil {
		slog.Error("got error from GetStockInfo", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return err
	}

	if price == nil { // если не передали кастомный price - используем актульный
		price = &stockInfo.Price
	}

	stockOperation := model.StockOperation{
		Ticker:     stockInfo.Ticker,
		Shortname:  stockInfo.Shortname,
		Quantity:   quantity,
		Price:      *price,
		TotalPrice: price.Mul(decimal.NewFromInt(int64(quantity))),
		Currency:   stockInfo.CurrencyID,
	}

	err = s.repo.InsertStockOperationToHistory(ctx, portfolioID, stockOperation)
	if err != nil {
		slog.Error("got error from repo.InsertStockOperationToHistory", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return err
	}

	return nil
}
