package investHelperService

import (
	"context"
	"errors"
	"log/slog"

	"github.com/KotFed0t/invest_helper_bot/internal/externalApi"
	"github.com/KotFed0t/invest_helper_bot/internal/model"
	"github.com/KotFed0t/invest_helper_bot/internal/model/moexModel"
	"github.com/KotFed0t/invest_helper_bot/internal/service"
	"github.com/KotFed0t/invest_helper_bot/utils"
)

type MoexApi interface {
	GetStocInfo(ctx context.Context, ticker string) (moexModel.StockInfo, error)
}

type Cache interface {
	GetStock(ctx context.Context, ticker string) (moexModel.StockInfo, error)
}

type Repository interface {
	RegUser(ctx context.Context, chatID int64) (userID int64, err error)
	CreateStocksPortfolio(ctx context.Context, name string, userID int64) (portfolioID int64, err error)
	GetUserID(ctx context.Context, chatID int64) (userID int64, err error)
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
	slog.Debug("RegUser start", slog.String("rqID", rqID), slog.Int64("chatID", chatID))

	_, err := s.repo.RegUser(ctx, chatID)
	if err != nil {
		slog.Error("got error from repo.RegUser", slog.String("rqID", rqID), slog.String("err", err.Error()))
		return err
	}

	slog.Debug("RegUser completed", slog.String("rqID", rqID), slog.Int64("chatID", chatID))

	return nil
}

func (s *InvestHelperService) CreateStocksPortfolio(ctx context.Context, portfolioName string, chatID int64) (portfolioID int64, err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	slog.Debug("CreateStocksPortfolio start", slog.String("rqID", rqID), slog.Int64("chatID", chatID))

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

	slog.Debug("CreateStocksPortfolio completed", slog.String("rqID", rqID))

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

func (s *InvestHelperService) GetStockInfo(ctx context.Context, ticker string) (model.Stock, error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	slog.Debug("GetStockInfo start", slog.String("rqID", rqID), slog.String("ticker", ticker))

	var (
		stockMoex moexModel.StockInfo
		err       error
	)

	stockMoex, err = s.cache.GetStock(ctx, ticker)
	if err != nil {
		slog.Warn("can't get stock info from cache", slog.String("rqID", rqID), slog.String("err", err.Error()))

		stockMoex, err = s.moexApi.GetStocInfo(ctx, ticker)
		if err != nil {
			if errors.Is(err, externalApi.ErrNotFound) {
				slog.Warn("stock not found in moexApi", slog.String("rqID", rqID))
				return model.Stock{}, service.ErrNotFound
			}
			slog.Error("can't get stock info from moexApi", slog.String("rqID", rqID), slog.String("err", err.Error()))
			return model.Stock{}, err
		}
	}

	if stockMoex.Status == false {
		return model.Stock{}, service.ErrStockNotActive
	}

	return model.Stock{
		Ticker:    stockMoex.Ticker,
		Shortname: stockMoex.Shortname,
		Lotsize:   stockMoex.Lotsize,
		Price:     stockMoex.Price,
	}, nil
}
