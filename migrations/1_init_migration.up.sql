CREATE TABLE IF NOT EXISTS users(
    user_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    chat_id BIGINT UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS portfolios(
    portfolio_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT,
    user_id BIGINT references users(user_id)
);

CREATE TABLE IF NOT EXISTS stocks_portfolio_details(
    portfolio_id BIGINT references portfolios(portfolio_id),
    ticker TEXT NOT NULL,
    weight DECIMAL(5, 2) NOT NULL DEFAULT 0,
    quantity INT NOT NULL DEFAULT 0,
    CONSTRAINT unique_portfolio_ticker UNIQUE (portfolio_id, ticker)
);

CREATE TABLE IF NOT EXISTS stocks_operations_history(
    portfolio_id BIGINT references portfolios(portfolio_id),
    ticker TEXT NOT NULL,
    shortname TEXT NOT NULL,
    quantity INT NOT NULL,
    price DECIMAL(18, 6) NOT NULL,
    total_price DECIMAL(18, 6) NOT NULL,
    currency TEXT NOT NULL,
    dt_create TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

CREATE INDEX CONCURRENTLY IF NOT EXISTS portfolios_user_id_idx ON portfolios(user_id);