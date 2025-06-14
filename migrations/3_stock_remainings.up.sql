CREATE TABLE IF NOT EXISTS stock_remainings(
    row_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    portfolio_id BIGINT NOT NULL references portfolios(portfolio_id) ON DELETE CASCADE,
    ticker TEXT NOT NULL,
    quantity INT NOT NULL,
    price DECIMAL(18, 6) NOT NULL,
    dt_create TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
    dt_update TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);