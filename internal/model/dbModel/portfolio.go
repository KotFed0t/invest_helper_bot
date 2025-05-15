package dbModel

type Portfolio struct {
	PortfolioID int64  `db:"portfolio_id"`
	Name        string `db:"name"`
}
