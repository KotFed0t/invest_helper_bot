package dbConverter

import (
	"github.com/KotFed0t/invest_helper_bot/internal/model"
	"github.com/KotFed0t/invest_helper_bot/internal/model/dbModel"
)

func ConvertStock(dbStock dbModel.Stock) model.StockBase {
	return model.StockBase{
		PortfolioID:  dbStock.PortfolioID,
		Ticker:       dbStock.Ticker,
		TargetWeight: dbStock.Weight,
		Quantity:     dbStock.Quantity,
	}
}
