package cache

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/KotFed0t/invest_helper_bot/config"
	"github.com/KotFed0t/invest_helper_bot/internal/model/moexModel"
	"github.com/KotFed0t/invest_helper_bot/utils"
	"github.com/redis/go-redis/v9"
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

		_, err = pipe.Set(ctx, stock.Ticker, stockJson, r.cfg.Cache.StocksExpiration).Result()
		if err != nil {
			slog.Error(
				"failed on pipe.Set",
				slog.String("rqID", rqID),
				slog.String("err", err.Error()),
				slog.Any("stock", stock),
			)
			return err
		}
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		slog.Error("failed on pipe.Exec", slog.String("rqID", rqID), slog.String("err", err.Error()))
	}

	slog.Debug("SetStocks completed", slog.String("rqID", rqID))

	return nil
}

func (r *RedisCache) GetStock(ctx context.Context, ticker string) (moexModel.StockInfo, error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	slog.Debug("GetStock start", slog.String("rqID", rqID))

	res, err := r.redis.Get(ctx, ticker).Result()
	if err != nil {
		slog.Error("failed on redis.Get", slog.String("rqID", rqID), slog.String("err", err.Error()), slog.String("key", ticker))
		return moexModel.StockInfo{}, err
	}

	stockInfo := moexModel.StockInfo{}
	err = json.Unmarshal([]byte(res), &stockInfo)
	if err != nil {
		slog.Error(
			"can't unmarshall stock in GetStock",
			slog.String("rqID", rqID),
			slog.String("err", err.Error()),
			slog.String("resultFromRedis", res),
		)
		return moexModel.StockInfo{}, errors.New("can't unmarshall stock")
	}

	slog.Debug("GetStock finished", slog.String("rqID", rqID))

	return stockInfo, nil
}