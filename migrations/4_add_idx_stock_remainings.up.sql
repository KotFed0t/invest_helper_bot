-- +no_transaction
CREATE INDEX CONCURRENTLY IF NOT EXISTS stock_remainings_portfolioid_ticker_idx ON stock_remainings(portfolio_id, ticker);