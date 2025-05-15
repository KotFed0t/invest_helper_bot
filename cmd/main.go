package main

import (
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
	"github.com/KotFed0t/invest_helper_bot/internal/scheduler"
	"github.com/KotFed0t/invest_helper_bot/internal/service/investHelperService"
	"github.com/KotFed0t/invest_helper_bot/internal/tgbot"
	"github.com/KotFed0t/invest_helper_bot/internal/transport/telegram"
)

func main() {
	cfg := config.MustLoad()

	setupLogger(cfg)

	slog.Debug("config", slog.Any("cfg", cfg))

	pgClient := data.NewPostgresClient(cfg)
	defer pgClient.Close()

	pgRepo := repository.NewPostgres(cfg, pgClient)

	redisClient := data.NewRedisClient(cfg)
	defer redisClient.Close()

	redisCache := cache.NewRedisCache(redisClient, cfg)
	redisSession := session.NewRedisSession(redisClient, cfg)

	moexApiClient := moexApi.New(cfg)

	investHelperSrv := investHelperService.New(cfg, pgRepo, redisCache, moexApiClient)

	sched := scheduler.New()
	sched.NewIntervalJob("fill moex cache", investHelperSrv.FillMoexCache, cfg.Jobs.FillMoexCacheInterval, true)
	sched.Start()
	defer sched.Stop()

	tgController := telegram.NewController(cfg, investHelperSrv, redisSession)

	tgBot := tgbot.New(cfg, tgController, redisSession)
	tgBot.Start()
	defer tgBot.Stop()

	// Waiting interruption signal
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-interrupt
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
