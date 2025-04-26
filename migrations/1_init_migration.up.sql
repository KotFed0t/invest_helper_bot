CREATE TABLE IF NOT EXISTS users(
    user_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    chat_id BIGINT UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS portfolios(
    portfolio_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT,
    user_id BIGINT references users(user_id)
);

CREATE TABLE IF NOT EXISTS stocks(
    stock_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ticker TEXT UNIQUE NOT NULL,
    shortname TEXT NOT NULL,
    lotsize INT NOT NULL,
    status BOOLEAN NOT NULL,
    currency TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS stocks_portfolio_details(
    portfolio_id BIGINT references portfolios(portfolio_id),
    stock_id BIGINT references stocks(stock_id),
    weight DECIMAL(5, 2) NOT NULL,
    user_id BIGINT references users(user_id),
    quantity INT NOT NULL,
    CONSTRAINT unique_portfolio_stock UNIQUE (portfolio_id, stock_id)
);

CREATE TABLE IF NOT EXISTS stocks_operations_history(
    portfolio_id BIGINT references portfolios(portfolio_id),
    stock_id BIGINT references stocks(stock_id),
    quantity INT NOT NULL,
    price DECIMAL(18, 6) NOT NULL,
    total_price DECIMAL(18, 6) NOT NULL,
    dt_create TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);
