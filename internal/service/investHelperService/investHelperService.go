package investHelperService

import (
	"context"
	"errors"
	"log/slog"
	"slices"

	"github.com/KotFed0t/invest_helper_bot/config"
	"github.com/KotFed0t/invest_helper_bot/data/repository"
	"github.com/KotFed0t/invest_helper_bot/internal/externalApi"
	"github.com/KotFed0t/invest_helper_bot/internal/model"
	"github.com/KotFed0t/invest_helper_bot/internal/model/moexModel"
	"github.com/KotFed0t/invest_helper_bot/internal/service"
	"github.com/KotFed0t/invest_helper_bot/utils"
	"github.com/shopspring/decimal"
)

type MoexApi interface {
	GetStocInfo(ctx context.Context, ticker string) (moexModel.StockInfo, error)
	GetStocsInfo(ctx context.Context, tickers []string) (map[string]moexModel.StockInfo, error)
	GetAllStocsInfo(ctx context.Context) ([]moexModel.StockInfo, error)
}

type Cache interface {
	GetStockInfo(ctx context.Context, ticker string) (moexModel.StockInfo, error)
	GetStocksInfo(ctx context.Context, tickers []string) (map[string]moexModel.StockInfo, error)
	GetPortfolioStock(ctx context.Context, ticker string, portfolioID int64) (model.Stock, error)
	GetPortfolioStocksForPage(ctx context.Context, portfolioID int64, page int) ([]model.Stock, error)
	GetPortfolioSummary(ctx context.Context, portfolioID int64) (model.PortfolioSummary, error)
	SetPortfolioStock(ctx context.Context, portfolioID int64, stock model.Stock) error
	SetPortfolioSummary(ctx context.Context, portfolioID int64, summary model.PortfolioSummary) error
	SetPortfolioStocksForPage(ctx context.Context, portfolioID int64, stocks []model.Stock, page int) error
	SetStocks(ctx context.Context, stocks []moexModel.StockInfo) error
	FlushPortfolioCache(ctx context.Context, portfolioID int64) error
	FlushPortfolioSummaryCache(ctx context.Context, portfolioID int64) error
	FlushPortfolioStocksPagesCache(ctx context.Context, portfolioID int64) error
}

type Repository interface {
	RegUser(ctx context.Context, chatID int64) (userID int64, err error)
	CreateStocksPortfolio(ctx context.Context, name string, userID int64) (portfolioID int64, err error)
	GetUserID(ctx context.Context, chatID int64) (userID int64, err error)
	GetStockFromPortfolio(ctx context.Context, ticker string, portfolioID int64) (stock model.StockBase, err error)
	GetStocksFromPortfolio(ctx context.Context, portfolioID int64) (stocks []model.StockBase, err error)
	GetOnlyInIndexStocksFromPortfolio(ctx context.Context, portfolioID int64) (stocks []model.StockBase, err error)
	GetPageStocksFromPortfolio(ctx context.Context, portfolioID int64, limit, offset int) (stocks []model.StockBase, err error)
	InsertStockToPortfolio(ctx context.Context, portfolioID int64, ticker string) (err error)
	DeleteStockFromPortfolio(ctx context.Context, portfolioID int64, ticker string) (err error)
	UpdatePortfolioStock(ctx context.Context, portfolioID int64, ticker string, weight *decimal.Decimal, quantity *int) (err error)
	InsertStockOperationToHistory(ctx context.Context, portfolioID int64, stockOperation model.StockOperation) (err error)
	GetPortfolioName(ctx context.Context, portfolioID int64) (name string, err error)
	GetPortfolios(ctx context.Context, chatID int64, limit, offset int) (portfolios []model.Portfolio, hasNextPage bool, err error)
	RebalanceWeights(ctx context.Context, portfolioID int64) (err error)
}

type InvestHelperService struct {
	cfg     *config.Config
	repo    Repository
	cache   Cache
	moexApi MoexApi
}

func New(cfg *config.Config, repo Repository, cache Cache, moexApi MoexApi) *InvestHelperService {
	return &InvestHelperService{
		cfg:     cfg,
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

func (s *InvestHelperService) getPortfolioStocksForPage(ctx context.Context, portfolioID int64, page int, portfolioBalance decimal.Decimal) ([]model.Stock, error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "InvestHelperService.getPortfolioStocksForPage"

	stocks, err := s.cache.GetPortfolioStocksForPage(ctx, portfolioID, page)
	if err == nil {
		slog.Info(
			"got stocks for page from cache",
			slog.String("rqID", rqID),
			slog.String("op", op),
			slog.Int64("portfolioID", portfolioID),
			slog.Int("page", page),
			slog.Any("stocks", stocks),
		)
		return stocks, nil
	}

	slog.Info("can't get stocks for page from cache", slog.String("rqID", rqID), slog.String("op", op), slog.Int64("portfolioID", portfolioID), slog.Int("page", page))

	// берем акции из БД
	stocksDb, err := s.repo.GetPageStocksFromPortfolio(ctx, portfolioID, s.cfg.StocksPerPage, (page-1)*s.cfg.StocksPerPage)
	if err != nil {
		slog.Error("got error from repo.GetPageStocksFromPortfolio", slog.String("rqID", rqID), slog.String("op", op))
		return nil, err
	}

	stocks, err = s.enrichStocks(ctx, stocksDb, portfolioBalance)
	if err != nil {
		return nil, err
	}

	// сохраняем в кэш
	go s.cache.SetPortfolioStocksForPage(context.WithoutCancel(ctx), portfolioID, stocks, page)

	return stocks, nil
}

func (s *InvestHelperService) GetPortfolioPage(ctx context.Context, portfolioID int64, page int) (model.PortfolioPage, error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "InvestHelperService.GetPortfolioInfoForPage"

	slog.Debug("GetPortfolioInfoForPage start", slog.String("rqID", rqID), slog.String("op", op), slog.Int64("portfolioID", portfolioID), slog.Int("page", page))
	defer func() {
		slog.Debug("GetPortfolioInfoForPage finished", slog.String("rqID", rqID), slog.String("op", op), slog.Int64("portfolioID", portfolioID), slog.Int("page", page))
	}()

	potfolioSummary, err := s.GetPortfolioSummaryInfo(ctx, portfolioID)
	if err != nil {
		return model.PortfolioPage{}, err
	}

	stocks, err := s.getPortfolioStocksForPage(ctx, portfolioID, page, potfolioSummary.TotalBalance)
	if err != nil {
		return model.PortfolioPage{}, err
	}

	portfolioPage := model.PortfolioPage{
		PortfolioSummary: potfolioSummary,
		CurPage:          page,
		Stocks:           stocks,
		TotalPages:       s.calculateTotalPages(potfolioSummary.StocksCount, s.cfg.StocksPerPage),
	}

	return portfolioPage, nil
}

func (s *InvestHelperService) calculateTotalPages(totalItems, itemsPerPage int) int {
	if itemsPerPage <= 0 {
		return 0
	}

	pages := totalItems / itemsPerPage
	if totalItems%itemsPerPage != 0 {
		pages++
	}
	return pages
}

func (s *InvestHelperService) GetStockInfo(ctx context.Context, ticker string) (stockInfo moexModel.StockInfo, err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "InvestHelperService.GetStockInfo"

	slog.Debug("GetStockInfo start", slog.String("rqID", rqID), slog.String("op", op), slog.String("ticker", ticker))
	defer func() {
		slog.Debug("GetStockInfo finished", slog.String("rqID", rqID), slog.String("op", op), slog.String("ticker", ticker))
	}()

	stockInfo, err = s.cache.GetStockInfo(ctx, ticker)
	if err == nil {
		return stockInfo, nil
	}

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

	return stockInfo, nil
}

func (s *InvestHelperService) getStocksInfo(ctx context.Context, tickers []string) (map[string]moexModel.StockInfo, error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "InvestHelperService.getStocksInfo"

	stocksInfoMap, err := s.cache.GetStocksInfo(ctx, tickers)
	if err == nil {
		slog.Debug("got stocksInfoMap from cache", slog.String("rqID", rqID), slog.String("op", op), slog.Any("stocksInfoMap", stocksInfoMap))
		return stocksInfoMap, nil
	}

	slog.Warn("can't get stocks info from cache", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))

	stocksInfoMap, err = s.moexApi.GetStocsInfo(ctx, tickers)
	if err != nil {
		slog.Error("got error from moexApi.GetStocsInfo", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return nil, err
	}
	slog.Debug("got stocksInfoMap from moexApi", slog.String("rqID", rqID), slog.String("op", op), slog.Any("stocksInfoMap", stocksInfoMap))

	return stocksInfoMap, nil
}

func (s *InvestHelperService) addStockToPortfolio(ctx context.Context, ticker string, portfolioID, chatID int64) error {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "InvestHelperService.addStockToPortfolio"

	err := s.repo.InsertStockToPortfolio(ctx, portfolioID, ticker)
	if err != nil {
		if errors.Is(err, repository.ErrAlreadyExists) {
			return nil
		}
		slog.Error("got error from repo.InsertStockToPortfolio", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return err
	}

	go s.cache.FlushPortfolioSummaryCache(context.WithoutCancel(ctx), portfolioID)
	go s.cache.FlushPortfolioStocksPagesCache(context.WithoutCancel(ctx), portfolioID)

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

	// селектим название портфеля
	name, err := s.repo.GetPortfolioName(ctx, portfolioID)
	if err != nil {
		slog.Warn("got error from repo.GetPortfolioName", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
	}
	summary.PortfolioName = name

	// получаем актуальные цены для акций
	tickers := make([]string, 0, len(stocks))
	for _, stock := range stocks {
		tickers = append(tickers, stock.Ticker)
	}

	stocksInfoMap, err := s.getStocksInfo(ctx, tickers)
	if err != nil {
		return model.PortfolioSummary{}, err
	}

	// считаем totalBalance и totalWeight
	for _, stock := range stocks {
		summary.TotalWeight = summary.TotalWeight.Add(stock.TargetWeight)

		stockInfo, ok := stocksInfoMap[stock.Ticker]
		if !ok {
			slog.Error("ticker not found in stocksInfoMap", slog.String("rqID", rqID), slog.String("op", op), slog.String("ticker", stock.Ticker))
			return model.PortfolioSummary{}, errors.New("can't calculate summary cause got partial info")
		}

		if !stock.TargetWeight.IsZero() {
			summary.TotalBalance = summary.TotalBalance.Add(stockInfo.Price.Mul(decimal.NewFromInt(int64(stock.Quantity))))
		} else {
			summary.BalanceOutsideIndex = summary.BalanceOutsideIndex.Add(stockInfo.Price.Mul(decimal.NewFromInt(int64(stock.Quantity))))
			summary.StocksOutsideIndexCnt++
		}
	}
	summary.StocksCount = len(stocks)

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
		return model.Stock{}, err
	}

	stock = model.Stock{
		StockBase:  stockDB,
		Shortname:  stockInfo.Shortname,
		Lotsize:    stockInfo.Lotsize,
		Price:      stockInfo.Price,
		TotalPrice: stockInfo.Price.Mul(decimal.NewFromInt(int64(stockDB.Quantity))),
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

func (s *InvestHelperService) deleteStockFromPortfolio(ctx context.Context, portfolioID int64, ticker string) error {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "InvestHelperService.deleteStockFromPortfolio"

	err := s.repo.DeleteStockFromPortfolio(ctx, portfolioID, ticker)
	if err != nil {
		slog.Error("got error from repo.DeleteStockFromPortfolio", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return err
	}

	err = s.cache.FlushPortfolioCache(ctx, portfolioID)
	if err != nil {
		slog.Error("got error from cache.FlushPortfolioCache", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
	}
	return nil
}

func (s *InvestHelperService) DeleteStockFromPortfolio(ctx context.Context, portfolioID int64, ticker string) error {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "InvestHelperService.DeleteStockFromPortfolio"

	slog.Debug("DeleteStock start", slog.String("rqID", rqID), slog.String("op", op), slog.String("ticker", ticker), slog.Int64("portfolioID", portfolioID))
	defer func() {
		slog.Debug("DeleteStock finished", slog.String("rqID", rqID), slog.String("op", op), slog.String("ticker", ticker), slog.Int64("portfolioID", portfolioID))
	}()

	err := s.deleteStockFromPortfolio(ctx, portfolioID, ticker)
	if err != nil {
		return err
	}

	return nil
}

func (s *InvestHelperService) enrichStocks(ctx context.Context, stocksDb []model.StockBase, portfolioBalance decimal.Decimal) ([]model.Stock, error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "InvestHelperService.enrichStocks"

	if len(stocksDb) == 0 {
		return []model.Stock{}, nil
	}

	tickers := make([]string, 0, len(stocksDb))
	for _, stock := range stocksDb {
		tickers = append(tickers, stock.Ticker)
	}

	stocksInfoMap, err := s.getStocksInfo(ctx, tickers)
	if err != nil {
		return nil, err
	}

	stocks := make([]model.Stock, 0, len(stocksDb))
	for _, stockDb := range stocksDb {
		stockInfo, ok := stocksInfoMap[stockDb.Ticker]
		if !ok {
			slog.Error("ticker not found in stocksInfoMap", slog.String("rqID", rqID), slog.String("op", op), slog.String("ticker", stockDb.Ticker))
			return nil, errors.New("can't calculate stocks for page cause got partial info")
		}

		stock := model.Stock{
			StockBase:  stockDb,
			Shortname:  stockInfo.Shortname,
			Lotsize:    stockInfo.Lotsize,
			Price:      stockInfo.Price,
			TotalPrice: stockInfo.Price.Mul(decimal.NewFromInt(int64(stockDb.Quantity))),
		}

		if !portfolioBalance.IsZero() {
			stock.ActualWeight = stock.TotalPrice.Div(portfolioBalance).Mul(decimal.NewFromInt(100))
		}

		stocks = append(stocks, stock)
	}

	return stocks, nil
}

func (s *InvestHelperService) CalculatePurchase(ctx context.Context, portfolioID int64, purchaseSum decimal.Decimal) ([]model.StockPurchase, error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "InvestHelperService.CalculatePurchase"

	slog.Debug("CalculatePurchase start", slog.String("rqID", rqID), slog.String("op", op), slog.String("purchaseSum", purchaseSum.StringFixed(2)), slog.Int64("portfolioID", portfolioID))
	defer func() {
		slog.Debug("CalculatePurchase finished", slog.String("rqID", rqID), slog.String("op", op), slog.String("purchaseSum", purchaseSum.StringFixed(2)), slog.Int64("portfolioID", portfolioID))
	}()

	// получить из БД акции где вес > 0
	stocksDb, err := s.repo.GetOnlyInIndexStocksFromPortfolio(ctx, portfolioID)
	if err != nil {
		slog.Error("got error from repo.GetOnlyInIndexStocksFromPortfolio", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return nil, err
	}

	if len(stocksDb) == 0 {
		return []model.StockPurchase{}, nil
	}

	// получить баланс портфеля
	portfolioSummary, err := s.GetPortfolioSummaryInfo(ctx, portfolioID)
	if err != nil {
		return nil, err
	}

	stocks, err := s.enrichStocks(ctx, stocksDb, portfolioSummary.TotalBalance)
	if err != nil {
		return nil, err
	}

	// сортируем список акций по убыванию цены лота
	slices.SortFunc(stocks, func(a, b model.Stock) int {
		aLotPrice := a.Price.Mul(decimal.NewFromInt(int64(a.Lotsize)))
		bLotPrice := b.Price.Mul(decimal.NewFromInt(int64(b.Lotsize)))

		switch {
		case aLotPrice.LessThan(bLotPrice):
			return 1
		case aLotPrice.GreaterThan(bLotPrice):
			return -1
		default:
			return 0
		}
	})

	// либо сортируем список акций по убыванию недокупленности относительно портфеля
	// slices.SortFunc(stocks, func(a, b model.Stock) int {
	// 	aWeightDiffrence := a.TargetWeight.Sub(a.ActualWeight)
	// 	bWeightDiffrence := b.TargetWeight.Sub(b.ActualWeight)

	// 	switch {
	// 	case aWeightDiffrence.LessThan(bWeightDiffrence):
	// 		return 1
	// 	case aWeightDiffrence.GreaterThan(bWeightDiffrence):
	// 		return -1
	// 	default:
	// 		return 0
	// 	}
	// })

	// итерируемся и заполняем stocksPurchase сначала целыми лотами и считаем общую сумму покупки целых лотов
	stocksToPurchase := make([]model.StockPurchase, 0, len(stocks))
	purchaseRemainder := purchaseSum
	for _, stock := range stocks {
		needToBuySum := portfolioSummary.TotalBalance.
			Add(purchaseSum).
			Mul(stock.TargetWeight).
			Div(decimal.NewFromInt(100)).
			Sub(stock.TotalPrice)

		if needToBuySum.LessThanOrEqual(decimal.NewFromInt(0)) {
			continue
		}

		lotPrice := stock.Price.Mul(decimal.NewFromInt(int64(stock.Lotsize)))
		if lotPrice.LessThanOrEqual(decimal.NewFromInt(0)) {
			continue
		}

		if purchaseRemainder.LessThan(needToBuySum) {
			needToBuySum = purchaseRemainder
		}

		lotsToBuy := needToBuySum.Div(lotPrice)
		wholeLots := lotsToBuy.IntPart()
		if wholeLots <= 0 {
			continue
		}

		stockToPurchase := model.StockPurchase{
			Ticker:       stock.Ticker,
			Shortname:    stock.Shortname,
			LotSize:      stock.Lotsize,
			LotsQuantity: lotsToBuy,
			StockPrice:   stock.Price,
		}
		stocksToPurchase = append(stocksToPurchase, stockToPurchase)
		purchaseRemainder = purchaseRemainder.Sub(stock.Price.Mul(decimal.NewFromInt(wholeLots * int64(stock.Lotsize))))
	}

	// теперь зная остаток средств после покупки целых лотов, итерируемся еще раз и считаем докупку остаточной части лотов (округляя математически)
	for i := range stocksToPurchase {
		purchaseStock := &stocksToPurchase[i]
		// округляем к целой части и проверяем в какую сторону округлилось
		if purchaseStock.LotsQuantity.Round(0).LessThanOrEqual(purchaseStock.LotsQuantity) {
			continue
		}

		// добавить еще +1 лот к покупке, если хватает остатка средств
		lotPrice := purchaseStock.StockPrice.Mul(decimal.NewFromInt(int64(purchaseStock.LotSize)))
		if purchaseRemainder.LessThan(lotPrice) {
			continue
		}

		purchaseStock.LotsQuantity = purchaseStock.LotsQuantity.Round(0)
		purchaseRemainder = purchaseRemainder.Sub(lotPrice)
	}

	slog.Info("result for purchase", slog.Any("purchaseStocks", stocksToPurchase))

	return stocksToPurchase, nil
}

func (s *InvestHelperService) GetPortfolios(ctx context.Context, chatID int64, page int) (portfolios []model.Portfolio, hasNextPage bool, err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "InvestHelperService.GetPortfolios"

	slog.Debug("GetPortfolios start", slog.String("rqID", rqID), slog.String("op", op), slog.Int64("chatID", chatID))
	defer func() {
		slog.Debug("GetPortfolios finished", slog.String("rqID", rqID), slog.String("op", op), slog.Int64("chatID", chatID))
	}()

	portfolios, hasNextPage, err = s.repo.GetPortfolios(ctx, chatID, s.cfg.PortfoliosPerPage, (page-1)*s.cfg.PortfoliosPerPage)
	if err != nil {
		slog.Error("got error from repo.GetPortfolios", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return nil, false, err
	}

	slog.Debug("GetPortfolios result", slog.Any("portfolios", portfolios))

	return portfolios, hasNextPage, nil
}

func (s *InvestHelperService) RebalanceWeights(ctx context.Context, portfolioID int64) error {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "InvestHelperService.RebalanceWeights"

	slog.Debug("RebalanceWeights start", slog.String("rqID", rqID), slog.String("op", op), slog.Int64("chatID", portfolioID))
	defer func() {
		slog.Debug("RebalanceWeights finished", slog.String("rqID", rqID), slog.String("op", op), slog.Int64("chatID", portfolioID))
	}()

	err := s.repo.RebalanceWeights(ctx, portfolioID)
	if err != nil {
		slog.Error("got error from repo.RebalanceWeights", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return err
	}

	err = s.cache.FlushPortfolioCache(ctx, portfolioID)
	if err != nil {
		slog.Error("got error from cache.FlushPortfolioCache", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
	}

	return nil
}

func (s *InvestHelperService) FillMoexCache(ctx context.Context) error {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "InvestHelperService.FillMoexCache"

	slog.Debug("FillMoexCache start", slog.String("rqID", rqID), slog.String("op", op))

	stocksInfo, err := s.moexApi.GetAllStocsInfo(ctx)
	if err != nil {
		slog.Error("initialFillCache failed on GetStocsInfo", slog.String("err", err.Error()))
		return err
	}

	err = s.cache.SetStocks(ctx, stocksInfo)
	if err != nil {
		slog.Error("initialFillCache failed on SetStocks", slog.String("err", err.Error()))
		return err
	}

	slog.Debug("FillMoexCache completed", slog.String("rqID", rqID), slog.String("op", op))

	return nil
}
