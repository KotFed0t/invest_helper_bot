package repository

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/KotFed0t/invest_helper_bot/config"
	"github.com/KotFed0t/invest_helper_bot/internal/model/moexModel"
	"github.com/KotFed0t/invest_helper_bot/utils"
	_ "github.com/jackc/pgx/v5/stdlib" // pgx driver
	"github.com/jmoiron/sqlx"
)

type Postgres struct {
	db  *sqlx.DB
	cfg *config.Config
}

func NewPostgres(cfg *config.Config, db *sqlx.DB) *Postgres {
	return &Postgres{db: db, cfg: cfg}
}

func (p *Postgres) UpdateStocksTable(ctx context.Context, stocksInfo []moexModel.StockInfo) (err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	slog.Debug("start UpdateStocksTable", slog.String("rqID", rqID))
	sb := strings.Builder{}
	args := make([]any, 0, len(stocksInfo)*5)

	defer func() {
		if err != nil {
			slog.Error("failed update stocs table", slog.String("rqID", rqID), slog.String("err", err.Error()))
		} else {
			slog.Debug("Update Stocks Table completed", slog.String("rqID", rqID))
		}
	}()

	sb.WriteString(`INSERT INTO stocks (ticker, shortname, lotsize, status, currency) VALUES `)

	for i, stock := range stocksInfo {
		args = append(args, stock.Ticker, stock.Shortname, stock.Lotsize, stock.Status, stock.CurrencyID)

		start := i*5 + 1
		sb.WriteString(fmt.Sprintf("($%d, $%d, $%d, $%d, $%d)",
			start, start+1, start+2, start+3, start+4,
		))

		if i < len(stocksInfo)-1 {
			sb.WriteString(",")
		}
	}

	sb.WriteString(`
		ON CONFLICT (ticker) DO UPDATE SET
			shortname = EXCLUDED.shortname,
			lotsize = EXCLUDED.lotsize,
			status = EXCLUDED.status,
			currency = EXCLUDED.currency;
	`)

	_, err = p.db.ExecContext(ctx, sb.String(), args...)
	return err
}

func (r *Postgres) RegUser(ctx context.Context, chatID int64) (userID int64, err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	query := `INSERT INTO users(chat_id) VALUES($1) RETURNING user_id`

	slog.Debug("RegUser start", slog.String("rqID", rqID), slog.String("query", query))
	defer func() {
		if err != nil {
			slog.Error("RegUser failed", slog.String("rqID", rqID), slog.String("err", err.Error()))
		} else {
			slog.Debug("RegUser completed", slog.String("rqID", rqID))
		}
	}()

	err = r.db.QueryRowContext(ctx, query, chatID).Scan(&userID)
	if err != nil {
		return 0, err
	}

	return userID, nil
}

func (r *Postgres) CreateStocksPortfolio(ctx context.Context, portfolioName string, userID int64) (portfolioID int64, err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	query := `INSERT INTO portfolios(name, user_id) VALUES($1, $2) RETURNING portfolio_id`

	slog.Debug("CreateStocksPortfolio start", slog.String("rqID", rqID), slog.String("query", query))
	defer func() {
		if err != nil {
			slog.Error("CreateStocksPortfolio failed", slog.String("rqID", rqID), slog.String("err", err.Error()))
		} else {
			slog.Debug("CreateStocksPortfolio completed", slog.String("rqID", rqID))
		}
	}()
	
	err = r.db.QueryRowContext(ctx, query, portfolioName, userID).Scan(&portfolioID)
	if err != nil {
		return 0, err
	}

	return portfolioID, nil
}

func (r *Postgres) GetUserID(ctx context.Context, chatID int64) (userID int64, err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	query := `SELECT user_id FROM users WHERE chat_id = $1`

	slog.Debug("GetUserID start", slog.String("rqID", rqID), slog.String("query", query))
	defer func() {
		if err != nil {
			slog.Error("GetUserID failed", slog.String("rqID", rqID), slog.String("err", err.Error()))
		} else {
			slog.Debug("GetUserID completed", slog.String("rqID", rqID))
		}
	}()

	err = r.db.QueryRowContext(ctx, query, chatID).Scan(&userID)
	if err != nil {
		return 0, err
	}

	return userID, nil
}
