package investHelperService

import (
	"context"
	"log/slog"

	"github.com/KotFed0t/invest_helper_bot/utils"
)

type Cache interface {
}

type Repository interface {
	RegUser(ctx context.Context, chatID int64) (userID int64, err error)
	CreateStocksPortfolio(ctx context.Context, name string, userID int64) (err error)
	GetUserID(ctx context.Context, chatID int64) (userID int64, err error)
}

type InvestHelperService struct {
	repo  Repository
	cache Cache
}

func New(repo Repository, cache Cache) *InvestHelperService {
	return &InvestHelperService{
		repo:  repo,
		cache: cache,
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

func (s *InvestHelperService) CreateStocksPortfolio(ctx context.Context, portfolioName string, chatID int64) error {
	rqID := utils.GetRequestIDFromCtx(ctx)
	slog.Debug("CreateStocksPortfolio start", slog.String("rqID", rqID), slog.Int64("chatID", chatID))

	userID, err := s.repo.GetUserID(ctx, chatID)
	if err != nil {
		slog.Error("got error from repo.GetUserID", slog.String("rqID", rqID), slog.String("err", err.Error()))
		return err
	}

	err = s.repo.CreateStocksPortfolio(ctx, portfolioName, userID)
	if err != nil {
		slog.Error("got error from repo.CreateStocksPortfolio", slog.String("rqID", rqID), slog.String("err", err.Error()))
		return err
	}

	slog.Debug("CreateStocksPortfolio completed", slog.String("rqID", rqID))

	return nil
}