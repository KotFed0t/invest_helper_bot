package session

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/KotFed0t/invest_helper_bot/config"
	"github.com/KotFed0t/invest_helper_bot/internal/model"
	"github.com/KotFed0t/invest_helper_bot/utils"
	"github.com/redis/go-redis/v9"
)

type RedisSession struct {
	redis *redis.Client
	cfg   *config.Config
}

func NewRedisSession(redisClient *redis.Client, cfg *config.Config) *RedisSession {
	return &RedisSession{redis: redisClient, cfg: cfg}
}

func (r *RedisSession) SetSession(ctx context.Context, key string, session model.Session) error {
	rqID := utils.GetRequestIDFromCtx(ctx)
	slog.Debug("start SetSession", slog.String("rqID", rqID), slog.Any("session", session))

	sessionJson, err := json.Marshal(session)
	if err != nil {
		slog.Error("can't marshall session", slog.String("rqID", rqID), slog.String("err", err.Error()), slog.Any("session", session))
		return errors.New("can't marshall session")
	}

	_, err = r.redis.Set(ctx, key, sessionJson, r.cfg.SessionExpiration).Result()
	if err != nil {
		slog.Error("failed on redis.Set", slog.String("rqID", rqID), slog.String("err", err.Error()), slog.Any("session", session))
		return err
	}

	slog.Debug("SetSession completed", slog.String("rqID", rqID))

	return nil
}

func (r *RedisSession) GetSession(ctx context.Context, key string) (model.Session, error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	slog.Debug("start GetSession", slog.String("rqID", rqID))

	res, err := r.redis.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			slog.Debug("session not found in redis", slog.String("rqID", rqID))
			return model.Session{}, ErrNotFound
		}
		
		slog.Error("failed on redis.Get", slog.String("rqID", rqID), slog.String("err", err.Error()), slog.Any("key", key))
		return model.Session{}, err
	}

	session := model.Session{}

	err = json.Unmarshal([]byte(res), &session)
	if err != nil {
		slog.Error("can't unmarshall session", slog.String("rqID", rqID), slog.String("err", err.Error()), slog.Any("resresultFromRedis", res))
		return model.Session{}, errors.New("can't unmarshall session")
	}

	slog.Debug("GetSession completed", slog.String("rqID", rqID), slog.Any("session", session))

	return session, nil
}