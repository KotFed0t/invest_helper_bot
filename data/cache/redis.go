package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/KotFed0t/invest_helper_bot/config"
	"github.com/KotFed0t/invest_helper_bot/internal/model"
	"github.com/KotFed0t/invest_helper_bot/internal/model/moexModel"
	"github.com/KotFed0t/invest_helper_bot/utils"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
)

type RedisCache struct {
	redis *redis.Client
	cfg   *config.Config
}

func NewRedisCache(redisClient *redis.Client, cfg *config.Config) *RedisCache {
	return &RedisCache{redis: redisClient, cfg: cfg}
}

func (r *RedisCache) SetStocks(ctx context.Context, stocks []moexModel.StockInfo) error {
	rqID := utils.GetRequestIDFromCtx(ctx)
	slog.Debug("start SetStocks", slog.String("rqID", rqID))

	pipe := r.redis.Pipeline()
	for _, stock := range stocks {
		stockJson, err := json.Marshal(stock)
		if err != nil {
			slog.Error(
				"can't marshall stock in SetStocks",
				slog.String("rqID", rqID),
				slog.String("err", err.Error()),
				slog.Any("stock", stock),
			)
			return errors.New("can't marshall stock")
		}

		_ = pipe.Set(ctx, stock.Ticker, stockJson, r.cfg.Cache.StocksExpiration)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		slog.Error("failed on pipe.Exec", slog.String("rqID", rqID), slog.String("err", err.Error()))
	}

	slog.Debug("SetStocks completed", slog.String("rqID", rqID))

	return nil
}

func (r *RedisCache) GetStockInfo(ctx context.Context, ticker string) (moexModel.StockInfo, error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	slog.Debug("GetStockInfo start", slog.String("rqID", rqID))

	res, err := r.redis.Get(ctx, ticker).Result()
	if err != nil {
		// TODO добавить проверку на redis.Nil чтобы писать warning а не error (везде)
		slog.Error("failed on redis.Get", slog.String("rqID", rqID), slog.String("err", err.Error()), slog.String("key", ticker))
		return moexModel.StockInfo{}, err
	}

	stockInfo := moexModel.StockInfo{}
	err = json.Unmarshal([]byte(res), &stockInfo)
	if err != nil {
		slog.Error(
			"can't unmarshall stock in GetStockInfo",
			slog.String("rqID", rqID),
			slog.String("err", err.Error()),
			slog.String("resultFromRedis", res),
		)
		return moexModel.StockInfo{}, errors.New("can't unmarshall stock")
	}

	slog.Debug("GetStockInfo finished", slog.String("rqID", rqID))

	return stockInfo, nil
}

func (r *RedisCache) GetStocksInfo(ctx context.Context, tickers []string) (map[string]moexModel.StockInfo, error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	slog.Debug("GetStocksInfo start", slog.String("rqID", rqID))

	values, err := r.redis.MGet(ctx, tickers...).Result()
	if err != nil {
		slog.Error("failed on redis.MGet", slog.String("rqID", rqID), slog.String("err", err.Error()), slog.Any("tickers", tickers))
		return nil, err
	}

	m := make(map[string]moexModel.StockInfo, len(values))
	for _, value := range values {
		if value == nil {
			return nil, ErrNotFound
		}

		jsonData, ok := value.(string)
		if !ok {
			return nil, errors.New("can't cast values from redis to string")
		}

		stockInfo := moexModel.StockInfo{}
		err = json.Unmarshal([]byte(jsonData), &stockInfo)
		if err != nil {
			return nil, errors.New("can't unmarshal json to stockInfo")
		}

		m[stockInfo.Ticker] = stockInfo
	}

	slog.Debug("GetStocksInfo finished", slog.String("rqID", rqID))

	return m, nil
}

func (r *RedisCache) GetPortfolioStock(ctx context.Context, ticker string, portfolioID int64) (model.Stock, error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	slog.Debug("GetPortfolioStock start", slog.String("rqID", rqID))

	key := r.createPortfolioStockKey(portfolioID, ticker)

	res, err := r.redis.Get(ctx, key).Result()
	if err != nil {
		slog.Error("failed on redis.Get", slog.String("rqID", rqID), slog.String("err", err.Error()), slog.String("key", ticker))
		return model.Stock{}, err
	}

	stock := model.Stock{}
	err = json.Unmarshal([]byte(res), &stock)
	if err != nil {
		slog.Error(
			"can't unmarshall stock in GetPortfolioStock",
			slog.String("rqID", rqID),
			slog.String("err", err.Error()),
			slog.String("resultFromRedis", res),
		)
		return model.Stock{}, errors.New("can't unmarshall stock")
	}

	slog.Debug("GetPortfolioStock finished", slog.String("rqID", rqID))

	return stock, nil
}

func (r *RedisCache) SetPortfolioStock(ctx context.Context, portfolioID int64, stock model.Stock) error {
	rqID := utils.GetRequestIDFromCtx(ctx)
	slog.Debug("SetPortfolioStock start", slog.String("rqID", rqID))

	key := r.createPortfolioStockKey(portfolioID, stock.Ticker)

	jsonData, err := json.Marshal(stock)
	if err != nil {
		slog.Error(
			"can't marshall stock in SetPortfolioStock",
			slog.String("rqID", rqID),
			slog.String("err", err.Error()),
			slog.Any("stock", stock),
		)
		return errors.New("can't marshall stock")
	}

	_, err = r.redis.Set(ctx, key, jsonData, r.cfg.Cache.StocksExpiration).Result()
	if err != nil {
		slog.Error("failed on redis.Set", slog.String("rqID", rqID), slog.String("err", err.Error()), slog.Any("stock", stock))
		return err
	}

	slog.Debug("SetPortfolioStock finished", slog.String("rqID", rqID))

	return nil
}

func (r *RedisCache) GetPortfolioSummary(ctx context.Context, portfolioID int64) (model.PortfolioSummary, error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	slog.Debug("GetPortfolioSummary start", slog.String("rqID", rqID))

	key := r.createPortfolioSummaryKey(portfolioID)

	res, err := r.redis.Get(ctx, key).Result()
	if err != nil {
		slog.Error("failed on redis.Get", slog.String("rqID", rqID), slog.String("err", err.Error()), slog.String("key", key))
		return model.PortfolioSummary{}, err
	}

	summary := model.PortfolioSummary{}
	err = json.Unmarshal([]byte(res), &summary)
	if err != nil {
		slog.Error(
			"can't unmarshall summary in GetPortfolioSummary",
			slog.String("rqID", rqID),
			slog.String("err", err.Error()),
			slog.String("resultFromRedis", res),
		)
		return model.PortfolioSummary{}, errors.New("can't unmarshall summary")
	}

	slog.Debug("GetPortfolioSummary finished", slog.String("rqID", rqID))

	return summary, nil
}

func (r *RedisCache) SetPortfolioSummary(ctx context.Context, portfolioID int64, summary model.PortfolioSummary) error {
	rqID := utils.GetRequestIDFromCtx(ctx)
	slog.Debug("RedisCache.SetPortfolioSummary start", slog.String("rqID", rqID), slog.Any("summary", summary))

	key := r.createPortfolioSummaryKey(portfolioID)

	jsonData, err := json.Marshal(summary)
	if err != nil {
		slog.Error(
			"can't marshall summary in RedisCache.SetPortfolioSummary",
			slog.String("rqID", rqID),
			slog.String("err", err.Error()),
			slog.Any("summary", summary),
		)
		return errors.New("can't marshall summary")
	}

	_, err = r.redis.Set(ctx, key, jsonData, r.cfg.Cache.StocksExpiration).Result()
	if err != nil {
		slog.Error("failed on redis.Set", slog.String("rqID", rqID), slog.String("err", err.Error()), slog.Any("summary", summary))
		return err
	}

	slog.Debug("RedisCache.SetPortfolioSummary finished", slog.String("rqID", rqID))

	return nil
}

func (r *RedisCache) createPortfolioSummaryKey(portfolioID int64) string {
	return fmt.Sprintf("portfolio:%s:summary", strconv.FormatInt(portfolioID, 10))
}

func (r *RedisCache) createPortfolioStockKey(portfolioID int64, ticker string) string {
	return fmt.Sprintf("portfolio:%s:ticker:%s", strconv.FormatInt(portfolioID, 10), ticker)
}

func (r *RedisCache) createPortfolioStocksPageKey(portfolioID int64, page int) string {
	return fmt.Sprintf("portfolio:%s:page:%s", strconv.FormatInt(portfolioID, 10), strconv.Itoa(page))
}

// avgPrice в начале, чтобы не сносился при flushPortolioCache
func (r *RedisCache) createPortfolioStockAvgPriceKey(portfolioID int64, ticker string) string {
	return fmt.Sprintf("avgPrice:portfolio:%s:ticker:%s", strconv.FormatInt(portfolioID, 10), ticker)
}

func (r *RedisCache) FlushPortfolioSummaryCache(ctx context.Context, portfolioID int64) error {
	rqID := utils.GetRequestIDFromCtx(ctx)
	slog.Debug("RedisCache.FlushPortfolioSummaryCache start", slog.String("rqID", rqID))

	key := r.createPortfolioSummaryKey(portfolioID)

	_, err := r.redis.Del(ctx, key).Result()
	if err != nil {
		slog.Error("failed on redis.Del", slog.String("rqID", rqID), slog.String("err", err.Error()), slog.Any("key", key))
	}

	slog.Debug("RedisCache.FlushPortfolioSummaryCache completed", slog.String("rqID", rqID))

	return nil
}

func (r *RedisCache) FlushPortfolioStocksPagesCache(ctx context.Context, portfolioID int64) error {
	rqID := utils.GetRequestIDFromCtx(ctx)
	slog.Debug("RedisCache.FlushPortfolioCache start", slog.String("rqID", rqID))

	pattern := fmt.Sprintf("portfolio:%s:page:*", strconv.FormatInt(portfolioID, 10))

	keys, err := r.redis.Keys(ctx, pattern).Result()
	if err != nil {
		slog.Error("failed on redis.Keys", slog.String("rqID", rqID), slog.String("err", err.Error()))
		return err
	}
	slog.Debug("got keys from redis.Keys", slog.String("rqID", rqID), slog.Any("keys", keys))

	_, err = r.redis.Del(ctx, keys...).Result()
	if err != nil {
		slog.Error("failed on redis.Del", slog.String("rqID", rqID), slog.String("err", err.Error()), slog.Any("keys", keys))
	}

	slog.Debug("RedisCache.FlushPortfolioStocksPages completed", slog.String("rqID", rqID))

	return nil
}

func (r *RedisCache) FlushPortfolioCache(ctx context.Context, portfolioID int64) error {
	rqID := utils.GetRequestIDFromCtx(ctx)
	slog.Debug("RedisCache.FlushPortfolioCache start", slog.String("rqID", rqID))

	pattern := fmt.Sprintf("portfolio:%s*", strconv.FormatInt(portfolioID, 10))

	keys, err := r.redis.Keys(ctx, pattern).Result()
	if err != nil {
		slog.Error("failed on redis.Keys", slog.String("rqID", rqID), slog.String("err", err.Error()))
		return err
	}
	slog.Debug("got keys from redis.Keys", slog.String("rqID", rqID), slog.Any("keys", keys))

	_, err = r.redis.Del(ctx, keys...).Result()
	if err != nil {
		slog.Error("failed on redis.Del", slog.String("rqID", rqID), slog.String("err", err.Error()), slog.Any("keys", keys))
	}

	slog.Debug("RedisCache.FlushPortfolioCache completed", slog.String("rqID", rqID))

	return nil
}

func (r *RedisCache) GetPortfolioStocksForPage(ctx context.Context, portfolioID int64, page int) ([]model.Stock, error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "RedisCache.GetPortfolioStocksForPage"

	slog.Debug("GetPortfolioStocksForPage start", slog.String("rqID", rqID), slog.String("op", op))
	defer func() {
		slog.Debug("GetPortfolioStocksForPage finished", slog.String("rqID", rqID), slog.String("op", op))
	}()

	key := r.createPortfolioStocksPageKey(portfolioID, page)

	res, err := r.redis.Get(ctx, key).Result()
	if err != nil {
		slog.Error("failed on redis.Get", slog.String("rqID", rqID), slog.String("err", err.Error()), slog.String("key", key))
		return nil, err
	}

	stocks := make([]model.Stock, 0)
	err = json.Unmarshal([]byte(res), &stocks)
	if err != nil {
		slog.Error(
			"can't unmarshall stocks",
			slog.String("rqID", rqID),
			slog.String("op", op),
			slog.String("err", err.Error()),
			slog.String("resultFromRedis", res),
		)
		return nil, errors.New("can't unmarshall stocks")
	}

	return stocks, nil
}

func (r *RedisCache) SetPortfolioStocksForPage(ctx context.Context, portfolioID int64, stocks []model.Stock, page int) error {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "RedisCache.SetPortfolioStocksForPage"

	slog.Debug("SetPortfolioStocksForPage start", slog.String("rqID", rqID), slog.String("op", op))
	defer func() {
		slog.Debug("SetPortfolioStocksForPage finished", slog.String("rqID", rqID), slog.String("op", op))
	}()

	key := r.createPortfolioStocksPageKey(portfolioID, page)

	jsonData, err := json.Marshal(stocks)
	if err != nil {
		slog.Error(
			"can't marshall stocks",
			slog.String("rqID", rqID),
			slog.String("op", op),
			slog.String("err", err.Error()),
			slog.Any("stocks", stocks),
		)
		return errors.New("can't marshall stocks")
	}

	_, err = r.redis.Set(ctx, key, jsonData, r.cfg.Cache.StocksExpiration).Result()
	if err != nil {
		slog.Error("failed on redis.Set", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return err
	}

	return nil
}

func (r *RedisCache) SetStockAvgPrices(ctx context.Context, portfolioID int64, stockAvgPrices ...model.StockAvgPrice) error {
	rqID := utils.GetRequestIDFromCtx(ctx)
	slog.Debug("start SetStockAvgPrices", slog.String("rqID", rqID))

	if len(stockAvgPrices) == 0 {
		slog.Warn("no data to SetStockAvgPrices")
		return nil
	}

	pipe := r.redis.Pipeline()
	for _, stock := range stockAvgPrices {
		key := r.createPortfolioStockAvgPriceKey(portfolioID, stock.Ticker)
		_ = pipe.Set(ctx, key, stock.AvgPrice, r.cfg.SessionExpiration)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		slog.Error("failed on pipe.Exec", slog.String("rqID", rqID), slog.String("err", err.Error()))
	}

	slog.Debug("SetStocks completed", slog.String("rqID", rqID))

	return nil
}

func (r *RedisCache) GetStockAvgPrice(ctx context.Context, portfolioID int64, ticker string) (decimal.Decimal, error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "RedisCache.GetStockAvgPrice"
	slog.Debug("RedisCache.GetStockAvgPrice start", slog.String("rqID", rqID))
	key := r.createPortfolioStockAvgPriceKey(portfolioID, ticker)

	res, err := r.redis.Get(ctx, key).Result()
	if err != nil {
		slog.Warn("failed on redis.Get", slog.String("rqID", rqID), slog.String("err", err.Error()), slog.String("key", ticker))
		return decimal.Decimal{}, err
	}

	avgPrice, err := decimal.NewFromString(res)
	if err != nil {
		slog.Error("incorrect avg price", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
	}

	slog.Debug("GetStockInfo finished", slog.String("rqID", rqID))

	return avgPrice, nil
}

func (r *RedisCache) GetStockAvgPrices(ctx context.Context, portfolioID int64, tickers ...string) (map[string]decimal.Decimal, error) {
	op := "RedisCache.GetStockAvgPrices"
	rqID := utils.GetRequestIDFromCtx(ctx)
	slog.Debug("GetStockAvgPrices start", slog.String("rqID", rqID))

	if len(tickers) == 0 {
		return nil, ErrNotFound
	}

	keys := make([]string, 0, len(tickers))
	for _, ticker := range tickers {
		keys = append(keys, r.createPortfolioStockAvgPriceKey(portfolioID, ticker))
	}

	values, err := r.redis.MGet(ctx, keys...).Result()
	if err != nil {
		slog.Error("failed on redis.MGet", slog.String("rqID", rqID), slog.String("err", err.Error()), slog.Any("op", op))
		return nil, err
	}

	m := make(map[string]decimal.Decimal, len(values))
	for i, value := range values {
		if value == nil {
			return nil, ErrNotFound
		}

		avgPriceStr, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("can't cast values from redis to string")
		}

		avgPrice, err := decimal.NewFromString(avgPriceStr)
		if err != nil {
			return nil, fmt.Errorf("can't create decimal from cache values: %w", err)
		}

		m[tickers[i]] = avgPrice
	}

	slog.Debug("GetStockAvgPrices finished", slog.String("rqID", rqID))

	return m, nil
}
