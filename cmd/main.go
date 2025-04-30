package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/KotFed0t/invest_helper_bot/config"
	"github.com/KotFed0t/invest_helper_bot/data"
	"github.com/KotFed0t/invest_helper_bot/data/cache"
	"github.com/KotFed0t/invest_helper_bot/data/repository"
	"github.com/KotFed0t/invest_helper_bot/data/session"
	"github.com/KotFed0t/invest_helper_bot/internal/externalApi/moexApi"
	"github.com/KotFed0t/invest_helper_bot/internal/service/investHelperService"
	"github.com/KotFed0t/invest_helper_bot/internal/tgbot"
	"github.com/KotFed0t/invest_helper_bot/internal/transport/telegram"
)

func main() {
	cfg := config.MustLoad()

	setupLogger(cfg)

	slog.Debug("config", slog.Any("cfg", cfg))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pgClient := data.NewPostgresClient(cfg)
	pgRepo := repository.NewPostgres(cfg, pgClient)

	redisClient := data.NewRedisClient(cfg)
	redisCache := cache.NewRedisCache(redisClient, cfg)
	redisSession := session.NewRedisSession(redisClient, cfg)

	moexApiClient := moexApi.New(cfg)

	go initialFillCache(ctx, moexApiClient, redisCache)

	investHelperSrv := investHelperService.New(pgRepo, redisCache)

	tgController := telegram.NewController(investHelperSrv, redisSession)

	tgBot := tgbot.New(cfg, tgController, redisSession)
	tgBot.Start()

	// Waiting interruption signal
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-interrupt

	// gracefull shutdown
	tgBot.Stop()
	_ = pgClient.Close()
	_ = redisClient.Close()
}

func setupLogger(cfg *config.Config) {
	var logLevel slog.Level

	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warning":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(log)
}

func initialFillCache(ctx context.Context, moexApiClient *moexApi.MoexApi, redisCache *cache.RedisCache) {
	slog.Info("start initial cache fiilling")

	stocksInfo, err := moexApiClient.GetStocsInfo(ctx)
	if err != nil {
		slog.Error("initialFillCache failed on GetStocsInfo", slog.String("err", err.Error()))
		return
	}

	err = redisCache.SetStocks(ctx, stocksInfo)
	if err != nil {
		slog.Error("initialFillCache failed on SetStocks", slog.String("err", err.Error()))
		return
	}

	slog.Info("cache filled successfully")
}
