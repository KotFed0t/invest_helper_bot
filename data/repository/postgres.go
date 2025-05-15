package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/KotFed0t/invest_helper_bot/config"
	"github.com/KotFed0t/invest_helper_bot/internal/converter/dbConverter"
	"github.com/KotFed0t/invest_helper_bot/internal/model"
	"github.com/KotFed0t/invest_helper_bot/internal/model/dbModel"
	"github.com/KotFed0t/invest_helper_bot/internal/model/moexModel"
	"github.com/KotFed0t/invest_helper_bot/utils"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib" // pgx driver
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
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

func (r *Postgres) GetStockFromPortfolio(ctx context.Context, ticker string, portfolioID int64) (stock model.StockBase, err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	query := `
		SELECT portfolio_id, ticker, weight, quantity
		FROM stocks_portfolio_details 
		WHERE portfolio_id = $1
		AND ticker = $2
		`

	slog.Debug("GetStockFromPortfolio start", slog.String("rqID", rqID), slog.String("query", query))
	defer func() {
		if err != nil {
			slog.Error("GetStockFromPortfolio failed", slog.String("rqID", rqID), slog.String("err", err.Error()))
		} else {
			slog.Debug("GetStockFromPortfolio completed", slog.String("rqID", rqID))
		}
	}()

	dbStock := dbModel.Stock{}
	err = r.db.QueryRowxContext(ctx, query, portfolioID, ticker).StructScan(&dbStock)
	if err != nil {
		return model.StockBase{}, err
	}

	return dbConverter.ConvertStock(dbStock), nil
}

func (r *Postgres) getStocksFromPortfolio(ctx context.Context, portfolioID int64, query string) (stocks []model.StockBase, err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	slog.Debug("getStocksFromPortfolio start", slog.String("rqID", rqID), slog.String("query", query))
	defer func() {
		if err != nil {
			slog.Error("getStocksFromPortfolio failed", slog.String("rqID", rqID), slog.String("err", err.Error()))
		} else {
			slog.Debug("getStocksFromPortfolio completed", slog.String("rqID", rqID))
		}
	}()

	rows, err := r.db.QueryxContext(ctx, query, portfolioID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var stock dbModel.Stock
		err = rows.StructScan(&stock)
		if err != nil {
			return nil, err
		}
		stocks = append(stocks, dbConverter.ConvertStock(stock))
	}

	return stocks, nil
}

func (r *Postgres) GetStocksFromPortfolio(ctx context.Context, portfolioID int64) (stocks []model.StockBase, err error) {
	query := `
		SELECT portfolio_id, ticker, weight, quantity
		FROM stocks_portfolio_details 
		WHERE portfolio_id = $1
		order by ticker 
		`

	return r.getStocksFromPortfolio(ctx, portfolioID, query)
}

func (r *Postgres) GetOnlyInIndexStocksFromPortfolio(ctx context.Context, portfolioID int64) (stocks []model.StockBase, err error) {
	query := `
		SELECT portfolio_id, ticker, weight, quantity
		FROM stocks_portfolio_details 
		WHERE portfolio_id = $1
		AND weight > 0
		`

	return r.getStocksFromPortfolio(ctx, portfolioID, query)
}

func (r *Postgres) InsertStockToPortfolio(ctx context.Context, portfolioID int64, ticker string) (err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	query := `INSERT INTO stocks_portfolio_details(portfolio_id, ticker) VALUES($1, $2)`

	slog.Debug("InsertStockToPortfolio start", slog.String("rqID", rqID), slog.String("query", query))
	defer func() {
		if err != nil {
			slog.Error("InsertStockToPortfolio failed", slog.String("rqID", rqID), slog.String("err", err.Error()))
		} else {
			slog.Debug("InsertStockToPortfolio completed", slog.String("rqID", rqID))
		}
	}()

	_, err = r.db.ExecContext(ctx, query, portfolioID, ticker)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" { // unique_violation
				return ErrAlreadyExists
			}
		}
		return err
	}

	return nil
}

func (r *Postgres) UpdatePortfolioStock(ctx context.Context, portfolioID int64, ticker string, weight *decimal.Decimal, quantity *int) (err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	query := `
		UPDATE stocks_portfolio_details
        SET 
            quantity = quantity + COALESCE($1, 0),
            weight = COALESCE($2, weight)
        WHERE 
			portfolio_id = $3
			AND ticker = $4
	`

	slog.Debug("Postgres.UpdatePortfolioStock start", slog.String("rqID", rqID), slog.String("query", query))
	defer func() {
		if err != nil {
			slog.Error("Postgres.UpdatePortfolioStock failed", slog.String("rqID", rqID), slog.String("err", err.Error()))
		} else {
			slog.Debug("Postgres.UpdatePortfolioStock completed", slog.String("rqID", rqID))
		}
	}()

	_, err = r.db.ExecContext(ctx, query, quantity, weight, portfolioID, ticker)
	if err != nil {
		return err
	}

	return nil
}

func (r *Postgres) InsertStockOperationToHistory(ctx context.Context, portfolioID int64, stockOperation model.StockOperation) (err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Postgres.InsertStockOperationToHistory"
	query := `
		INSERT INTO stocks_operations_history(portfolio_id, ticker, shortname, quantity, price, total_price, currency)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	slog.Debug(
		"InsertStockOperationToHistory start",
		slog.String("rqID", rqID),
		slog.String("op", op),
		slog.Int64("portolioID", portfolioID),
		slog.Any("stockOperation", stockOperation),
		slog.String("query", query),
	)
	defer func() {
		if err != nil {
			slog.Error("InsertStockOperationToHistory failed", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		} else {
			slog.Debug("InsertStockOperationToHistory completed", slog.String("rqID", rqID), slog.String("op", op))
		}
	}()

	_, err = r.db.ExecContext(
		ctx,
		query,
		portfolioID,
		stockOperation.Ticker,
		stockOperation.Shortname,
		stockOperation.Quantity,
		stockOperation.Price,
		stockOperation.TotalPrice,
		stockOperation.Currency,
	)

	if err != nil {
		return err
	}
	return nil
}

func (r *Postgres) GetPageStocksFromPortfolio(ctx context.Context, portfolioID int64, limit, offset int) (stocks []model.StockBase, err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Postgres.GetPageStocksFromPortfolio"
	params := map[string]any{
		"portfolioID": portfolioID,
		"limit":       limit,
		"offset":      offset,
	}
	query := `
		SELECT portfolio_id, ticker, weight, quantity
		FROM stocks_portfolio_details 
		WHERE portfolio_id = $1
		ORDER BY ticker
		LIMIT $2
		OFFSET $3
		`

	slog.Debug("GetPageStocksFromPortfolio start", slog.String("rqID", rqID), slog.String("op", op), slog.String("query", query), slog.Any("params", params))
	defer func() {
		if err != nil {
			slog.Error("GetPageStocksFromPortfolio failed", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		} else {
			slog.Debug("GetPageStocksFromPortfolio completed", slog.String("rqID", rqID), slog.String("op", op))
		}
	}()

	rows, err := r.db.QueryxContext(ctx, query, portfolioID, limit, offset)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	stocks = make([]model.StockBase, 0, limit)
	for rows.Next() {
		var stock dbModel.Stock
		err = rows.StructScan(&stock)
		if err != nil {
			return nil, err
		}
		stocks = append(stocks, dbConverter.ConvertStock(stock))
	}

	return stocks, nil
}

func (r *Postgres) DeleteStockFromPortfolio(ctx context.Context, portfolioID int64, ticker string) (err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Postgres.DeleteStockFromPortfolio"
	params := map[string]any{
		"portfolioID": portfolioID,
		"ticker":      ticker,
	}

	query := `
		DELETE FROM stocks_portfolio_details 
		WHERE 
			portfolio_id = $1
			AND ticker = $2
		`

	slog.Debug("DeleteStockFromPortfolio start", slog.String("rqID", rqID), slog.String("op", op), slog.String("query", query), slog.Any("params", params))
	defer func() {
		if err != nil {
			slog.Error("DeleteStockFromPortfolio failed", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		} else {
			slog.Debug("DeleteStockFromPortfolio completed", slog.String("rqID", rqID), slog.String("op", op))
		}
	}()

	_, err = r.db.ExecContext(ctx, query, portfolioID, ticker)
	if err != nil {
		return err
	}

	return nil
}

func (r *Postgres) GetPortfolioName(ctx context.Context, portfolioID int64) (name string, err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Postgres.GetPortfolioName"
	params := map[string]any{
		"portfolioID": portfolioID,
	}

	query := `
		SELECT name FROM portfolios 
		WHERE portfolio_id = $1
		`

	slog.Debug("GetPortfolioName start", slog.String("rqID", rqID), slog.String("op", op), slog.String("query", query), slog.Any("params", params))
	defer func() {
		if err != nil {
			slog.Error("GetPortfolioName failed", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		} else {
			slog.Debug("GetPortfolioName completed", slog.String("rqID", rqID), slog.String("op", op))
		}
	}()

	err = r.db.QueryRowxContext(ctx, query, portfolioID).Scan(&name)
	if err != nil {
		return "", err
	}

	return name, nil
}

func (r *Postgres) GetPortfolios(ctx context.Context, chatID int64, limit, offset int) (portfolios []model.Portfolio, hasNextPage bool, err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Postgres.GetPortfolios"
	params := map[string]any{
		"chatID": chatID,
		"limit":  limit,
		"offset": offset,
	}
	query := `
		select p.portfolio_id, p."name" from portfolios p
		join users u using(user_id)
		where u.chat_id = $1
		limit $2
		offset $3
		`

	slog.Debug("GetPortfolios start", slog.String("rqID", rqID), slog.String("op", op), slog.String("query", query), slog.Any("params", params))
	defer func() {
		if err != nil {
			slog.Error("GetPortfolios failed", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		} else {
			slog.Debug("GetPortfolios completed", slog.String("rqID", rqID), slog.String("op", op))
		}
	}()

	// выбираем на 1 больше, чтобы знать есть ли next page
	rows, err := r.db.QueryxContext(ctx, query, chatID, limit+1, offset)
	if err != nil {
		return nil, false, err
	}

	defer rows.Close()

	i := 0
	portfolios = make([]model.Portfolio, 0, limit)
	for rows.Next() {
		i++
		var portfolio dbModel.Portfolio
		err = rows.StructScan(&portfolio)
		if err != nil {
			return nil, false, err
		}

		if i > limit { // если на 1 больше лимита, значит есть next page
			hasNextPage = true
			break
		}
		portfolios = append(portfolios, dbConverter.ConvertPortfolio(portfolio))
	}

	return portfolios, hasNextPage, nil
}

func (r *Postgres) RebalanceWeights(ctx context.Context, portfolioID int64) (err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Postgres.RebalanceWeights"
	params := map[string]any{
		"portfolioID": portfolioID,
	}

	query := `
		WITH total AS (
			SELECT SUM(weight) as total_weight
			FROM stocks_portfolio_details
			WHERE portfolio_id = $1 AND weight > 0
		)

		UPDATE stocks_portfolio_details s
		SET weight = (s.weight / t.total_weight) * 100
		FROM total t
		WHERE s.portfolio_id = $1 AND s.weight > 0;
		`

	slog.Debug("RebalanceWeights start", slog.String("rqID", rqID), slog.String("op", op), slog.String("query", query), slog.Any("params", params))
	defer func() {
		if err != nil {
			slog.Error("RebalanceWeights failed", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		} else {
			slog.Debug("RebalanceWeights completed", slog.String("rqID", rqID), slog.String("op", op))
		}
	}()

	_, err = r.db.ExecContext(ctx, query, portfolioID)
	if err != nil {
		return err
	}

	return nil
}
