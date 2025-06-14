package postgres

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/KotFed0t/invest_helper_bot/data/repository"
	"github.com/KotFed0t/invest_helper_bot/internal/converter/dbConverter"
	"github.com/KotFed0t/invest_helper_bot/internal/model"
	"github.com/KotFed0t/invest_helper_bot/internal/model/dbModel"
	"github.com/KotFed0t/invest_helper_bot/utils"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib" // pgx driver
	"github.com/shopspring/decimal"
)

func (r *Postgres) InsertUser(ctx context.Context, chatID int64) (userID int64, err error) {
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

	err = r.txOrDb(ctx).QueryRowContext(ctx, query, chatID).Scan(&userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" { // unique_violation
				return 0, repository.ErrAlreadyExists
			}
		}
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

	err = r.txOrDb(ctx).QueryRowContext(ctx, query, portfolioName, userID).Scan(&portfolioID)
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

	err = r.txOrDb(ctx).QueryRowContext(ctx, query, chatID).Scan(&userID)
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
	err = r.txOrDb(ctx).QueryRowxContext(ctx, query, portfolioID, ticker).StructScan(&dbStock)
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

	rows, err := r.txOrDb(ctx).QueryxContext(ctx, query, portfolioID)
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

	_, err = r.txOrDb(ctx).ExecContext(ctx, query, portfolioID, ticker)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" { // unique_violation
				return repository.ErrAlreadyExists
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

	_, err = r.txOrDb(ctx).ExecContext(ctx, query, quantity, weight, portfolioID, ticker)
	if err != nil {
		return err
	}

	return nil
}

func (r *Postgres) InsertStockOperationToHistory(ctx context.Context, portfolioID int64, stockOperation model.StockOperation) (err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Postgres.InsertStockOperationToHistory"
	query := `
		INSERT INTO stocks_operations_history(portfolio_id, ticker, shortname, quantity, price, total_price, currency, dt_create)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
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

	_, err = r.txOrDb(ctx).ExecContext(
		ctx,
		query,
		portfolioID,
		stockOperation.Ticker,
		stockOperation.Shortname,
		stockOperation.Quantity,
		stockOperation.Price,
		stockOperation.TotalPrice,
		stockOperation.Currency,
		stockOperation.DtCreate,
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

	rows, err := r.txOrDb(ctx).QueryxContext(ctx, query, portfolioID, limit, offset)
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

	_, err = r.txOrDb(ctx).ExecContext(ctx, query, portfolioID, ticker)
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

	err = r.txOrDb(ctx).QueryRowxContext(ctx, query, portfolioID).Scan(&name)
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
	rows, err := r.txOrDb(ctx).QueryxContext(ctx, query, chatID, limit+1, offset)
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

	_, err = r.txOrDb(ctx).ExecContext(ctx, query, portfolioID)
	if err != nil {
		return err
	}

	return nil
}

func (r *Postgres) DeletePortfolio(ctx context.Context, portfolioID int64) (err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Postgres.DeletePortfolio"
	params := map[string]any{
		"portfolioID": portfolioID,
	}

	// каскадное удаление
	query := `
		DELETE FROM portfolios 
		WHERE portfolio_id = $1
		`

	slog.Debug("DeletePortfolio start", slog.String("rqID", rqID), slog.String("op", op), slog.String("query", query), slog.Any("params", params))
	defer func() {
		if err != nil {
			slog.Error("DeletePortfolio failed", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		} else {
			slog.Debug("DeletePortfolio completed", slog.String("rqID", rqID), slog.String("op", op))
		}
	}()

	_, err = r.txOrDb(ctx).ExecContext(ctx, query, portfolioID)
	if err != nil {
		return err
	}

	return nil
}

func (r *Postgres) GetAllStocksByUserID(ctx context.Context, userID int64) (stocksByPortfolios map[int64][]model.StockBase, err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Postgres.GetAllStocksByUserID"
	params := map[string]any{
		"userID": userID,
	}
	query := `
		select portfolio_id, ticker, weight, quantity from portfolios
		join stocks_portfolio_details using(portfolio_id)
		where user_id = $1
		`

	slog.Debug("GetAllStocksByUserID start", slog.String("rqID", rqID), slog.String("op", op), slog.String("query", query), slog.Any("params", params))
	defer func() {
		if err != nil {
			slog.Error("GetAllStocksByUserID failed", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		} else {
			slog.Debug("GetAllStocksByUserID completed", slog.String("rqID", rqID), slog.String("op", op))
		}
	}()

	rows, err := r.txOrDb(ctx).QueryxContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	stocksByPortfolios = make(map[int64][]model.StockBase)
	for rows.Next() {
		var stock dbModel.Stock
		err = rows.StructScan(&stock)
		if err != nil {
			return nil, err
		}
		stocsSlice := stocksByPortfolios[stock.PortfolioID]
		stocsSlice = append(stocsSlice, dbConverter.ConvertStock(stock))
		stocksByPortfolios[stock.PortfolioID] = stocsSlice
	}

	return stocksByPortfolios, nil
}

func (r *Postgres) GetAllStockOperationsByUserID(ctx context.Context, userID int64) (stockOperationsByPortfolios map[int64][]model.StockOperation, err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Postgres.GetAllStockOperationsByUserID"
	params := map[string]any{
		"userID": userID,
	}
	query := `
		select portfolio_id, ticker, shortname, quantity, price, total_price, currency, dt_create from portfolios
		join stocks_operations_history using(portfolio_id)
		where user_id = $1
		`

	slog.Debug("GetAllStockOperationsByUserID start", slog.String("rqID", rqID), slog.String("op", op), slog.String("query", query), slog.Any("params", params))
	defer func() {
		if err != nil {
			slog.Error("GetAllStockOperationsByUserID failed", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		} else {
			slog.Debug("GetAllStockOperationsByUserID completed", slog.String("rqID", rqID), slog.String("op", op))
		}
	}()

	rows, err := r.txOrDb(ctx).QueryxContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	stockOperationsByPortfolios = make(map[int64][]model.StockOperation)
	for rows.Next() {
		var stockOperation dbModel.StockOperation
		err = rows.StructScan(&stockOperation)
		if err != nil {
			return nil, err
		}
		stockOperationsSlice := stockOperationsByPortfolios[stockOperation.PortfolioID]
		stockOperationsSlice = append(stockOperationsSlice, dbConverter.ConvertStockOperation(stockOperation))
		stockOperationsByPortfolios[stockOperation.PortfolioID] = stockOperationsSlice
	}

	return stockOperationsByPortfolios, nil
}

func (r *Postgres) GetAllPortfolioNamesByUserID(ctx context.Context, userID int64) (portfolioNames map[int64]string, err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Postgres.GetAllPortfolioNamesByUserID"
	params := map[string]any{
		"userID": userID,
	}
	query := `
		select portfolio_id, name from portfolios
		where user_id = $1
		`

	slog.Debug("GetAllPortfolioNamesByUserID start", slog.String("rqID", rqID), slog.String("op", op), slog.String("query", query), slog.Any("params", params))
	defer func() {
		if err != nil {
			slog.Error("GetAllPortfolioNamesByUserID failed", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		} else {
			slog.Debug("GetAllPortfolioNamesByUserID completed", slog.String("rqID", rqID), slog.String("op", op))
		}
	}()

	rows, err := r.txOrDb(ctx).QueryxContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	portfolioNames = make(map[int64]string)
	for rows.Next() {
		var portfolio dbModel.Portfolio
		err = rows.StructScan(&portfolio)
		if err != nil {
			return nil, err
		}
		portfolioNames[portfolio.PortfolioID] = portfolio.Name
	}

	return portfolioNames, nil
}

func (r *Postgres) UpdateQuantityPortfolioStocks(ctx context.Context, portfolioID int64, stocks []model.StockOperation) (err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Postgres.UpdateQuantityPortfolioStocks"
	params := map[string]any{
		"portfolioID": portfolioID,
		"stocks":      stocks,
	}

	query := `
		UPDATE stocks_portfolio_details AS s
		SET quantity = s.quantity + u.quantity
		FROM UNNEST($1::text[], $2::int[]) AS u(ticker, quantity)
		WHERE s.ticker = u.ticker AND s.portfolio_id = $3
		`

	tickers := make([]string, 0, len(stocks))
	quantities := make([]int, 0, len(stocks))
	for _, stock := range stocks {
		tickers = append(tickers, stock.Ticker)
		quantities = append(quantities, stock.Quantity)
	}

	slog.Debug("UpdateQuantityPortfolioStocks start", slog.String("rqID", rqID), slog.String("op", op), slog.String("query", query), slog.Any("params", params))
	defer func() {
		if err != nil {
			slog.Error("UpdateQuantityPortfolioStocks failed", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		} else {
			slog.Debug("UpdateQuantityPortfolioStocks completed", slog.String("rqID", rqID), slog.String("op", op))
		}
	}()

	_, err = r.txOrDb(ctx).ExecContext(ctx, query, tickers, quantities, portfolioID)
	if err != nil {
		return err
	}

	return nil
}

func (r *Postgres) InsertStockOperationsToHistory(ctx context.Context, portfolioID int64, stockOperations []model.StockOperation) (err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Postgres.InsertStockOperationsToHistory"
	params := map[string]any{
		"portfolioID":     portfolioID,
		"stockOperations": stockOperations,
	}
	query := `
        INSERT INTO stocks_operations_history(
            portfolio_id, ticker, shortname, quantity,
            price, total_price, currency, dt_create
        )
        SELECT 
            $1, -- portfolio_id
            u.ticker, 
            u.shortname, 
            u.quantity,
            u.price, 
            u.total_price, 
            $2, -- currency
            u.dt_create
        FROM UNNEST(
            $3::text[],
            $4::text[],
            $5::integer[],
            $6::decimal[],
            $7::decimal[],
            $8::timestamptz[]
        ) AS u(ticker, shortname, quantity, price, total_price, dt_create)`

	tickers := make([]string, 0, len(stockOperations))
	shortNames := make([]string, 0, len(stockOperations))
	quantities := make([]int, 0, len(stockOperations))
	prices := make([]decimal.Decimal, 0, len(stockOperations))
	totalPrices := make([]decimal.Decimal, 0, len(stockOperations))
	dtCreates := make([]time.Time, 0, len(stockOperations))

	for _, op := range stockOperations {
		tickers = append(tickers, op.Ticker)
		shortNames = append(shortNames, op.Shortname)
		quantities = append(quantities, op.Quantity)
		prices = append(prices, op.Price)
		totalPrices = append(totalPrices, op.TotalPrice)
		dtCreates = append(dtCreates, op.DtCreate)
	}

	slog.Debug(
		"InsertStockOperationsToHistory start",
		slog.String("rqID", rqID),
		slog.String("op", op),
		slog.String("query", query),
		slog.Any("params", params),
	)
	defer func() {
		if err != nil {
			slog.Error("InsertStockOperationsToHistory failed", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		} else {
			slog.Debug("InsertStockOperationsToHistory completed", slog.String("rqID", rqID), slog.String("op", op))
		}
	}()

	_, err = r.txOrDb(ctx).ExecContext(
		ctx,
		query,
		portfolioID,
		"RUB",
		tickers,
		shortNames,
		quantities,
		prices,
		totalPrices,
		dtCreates,
	)

	if err != nil {
		return err
	}
	return nil
}

func (r *Postgres) GetAverageStockPurchasePrice(ctx context.Context, portfolioID int64, ticker string) (avgPrice decimal.Decimal, err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Postgres.GetAverageStockPurchasePrice"
	params := map[string]any{
		"portfolioID": portfolioID,
		"ticker":      ticker,
	}
	query := `
		SELECT SUM(quantity * price)/NULLIF(SUM(quantity), 0) as avg_price
		WHERE portfolio_id = $1
		AND ticker = $2
        `

	slog.Debug(
		"GetAverageStockPurchasePrice start",
		slog.String("rqID", rqID),
		slog.String("op", op),
		slog.String("query", query),
		slog.Any("params", params),
	)
	defer func() {
		if err != nil {
			slog.Error("GetAverageStockPurchasePrice failed", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		} else {
			slog.Debug("GetAverageStockPurchasePrice completed", slog.String("rqID", rqID), slog.String("op", op))
		}
	}()

	err = r.txOrDb(ctx).QueryRowxContext(ctx, query, portfolioID, ticker).Scan(&avgPrice)

	if err != nil {
		return decimal.Decimal{}, err
	}
	return avgPrice, nil
}

func (r *Postgres) GetAverageStockPurchasePrices(ctx context.Context, portfolioID int64) (avgPrices map[string]decimal.Decimal, err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Postgres.GetAverageStockPurchasePrices"
	params := map[string]any{
		"portfolioID": portfolioID,
	}
	query := `
		SELECT ticker, SUM(quantity * price)/NULLIF(SUM(quantity), 0) as avg_price
		WHERE portfolio_id = $1
		GROUP BY ticker
        `

	slog.Debug(
		"GetAverageStockPurchasePrices start",
		slog.String("rqID", rqID),
		slog.String("op", op),
		slog.String("query", query),
		slog.Any("params", params),
	)
	defer func() {
		if err != nil {
			slog.Error("GetAverageStockPurchasePrices failed", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		} else {
			slog.Debug("GetAverageStockPurchasePrices completed", slog.String("rqID", rqID), slog.String("op", op))
		}
	}()

	rows, err := r.txOrDb(ctx).QueryxContext(ctx, query, portfolioID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	avgPrices = make(map[string]decimal.Decimal)
	for rows.Next() {
		var ticker string
		var avgPrice decimal.Decimal
		err = rows.Scan(&ticker, &avgPrice)
		if err != nil {
			return nil, err
		}
		avgPrices[ticker] = avgPrice
	}
	return avgPrices, nil
}

func (r *Postgres) InsertStockOperationToRemainings(ctx context.Context, portfolioID int64, stockOperation model.StockOperation) (err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Postgres.InsertStockOperationToRemainings"
	query := `
		INSERT INTO stock_remainings(portfolio_id, ticker, quantity, price)
		VALUES ($1, $2, $3, $4, $5)
	`

	slog.Debug(
		"InsertStockOperationToRemainings start",
		slog.String("rqID", rqID),
		slog.String("op", op),
		slog.Int64("portolioID", portfolioID),
		slog.Any("stockOperation", stockOperation),
		slog.String("query", query),
	)
	defer func() {
		if err != nil {
			slog.Error("InsertStockOperationToRemainings failed", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		} else {
			slog.Debug("InsertStockOperationToRemainings completed", slog.String("rqID", rqID), slog.String("op", op))
		}
	}()

	_, err = r.txOrDb(ctx).ExecContext(
		ctx,
		query,
		portfolioID,
		stockOperation.Ticker,
		stockOperation.Quantity,
		stockOperation.Price,
	)

	if err != nil {
		return err
	}
	return nil
}

func (r *Postgres) InsertStockOperationsToRemainings(ctx context.Context, portfolioID int64, stockOperations []model.StockOperation) (err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "Postgres.InsertStockOperationsToRemainings"
	params := map[string]any{
		"portfolioID":     portfolioID,
		"stockOperations": stockOperations,
	}
	query := `
        INSERT INTO stock_remainings(
            portfolio_id, ticker, quantity, price
        )
        SELECT 
            $1, -- portfolio_id
            u.ticker, 
            u.quantity,
            u.price
        FROM UNNEST(
            $2::text[],
            $3::integer[],
            $4::decimal[],
        ) AS u(ticker, quantity, price)`

	tickers := make([]string, 0, len(stockOperations))
	quantities := make([]int, 0, len(stockOperations))
	prices := make([]decimal.Decimal, 0, len(stockOperations))

	for _, op := range stockOperations {
		tickers = append(tickers, op.Ticker)
		quantities = append(quantities, op.Quantity)
		prices = append(prices, op.Price)
	}

	slog.Debug(
		"InsertStockOperationsToRemainings start",
		slog.String("rqID", rqID),
		slog.String("op", op),
		slog.String("query", query),
		slog.Any("params", params),
	)
	defer func() {
		if err != nil {
			slog.Error("InsertStockOperationsToRemainings failed", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		} else {
			slog.Debug("InsertStockOperationsToRemainings completed", slog.String("rqID", rqID), slog.String("op", op))
		}
	}()

	_, err = r.txOrDb(ctx).ExecContext(
		ctx,
		query,
		portfolioID,
		tickers,
		quantities,
		prices,
	)

	if err != nil {
		return err
	}
	return nil
}

// селект операций по тикеру отсортированным по дате

// (по row_id) update единичный (так как все остальное удаляем и только 1 будет апдейтится)

// (по row_id) delete для нескольких

// по кэшированию пока хз как лучше. По сути просто все сразу кэшировать (мапу), так как нам надо удостоверяться что именно акции нет, а с единичными так не получится.