-- +no_transaction
CREATE INDEX CONCURRENTLY IF NOT EXISTS portfolios_user_id_idx ON portfolios(user_id);