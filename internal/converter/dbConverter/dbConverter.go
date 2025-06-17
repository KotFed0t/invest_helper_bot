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

func ConvertStockOperation(dbStock dbModel.StockOperation) model.StockOperation {
	return model.StockOperation{
		Ticker:     dbStock.Ticker,
		Shortname:  dbStock.Shortname,
		Quantity:   dbStock.Quantity,
		Price:      dbStock.Price,
		TotalPrice: dbStock.TotalPrice,
		Currency:   dbStock.Currency,
		DtCreate:   dbStock.DtCreate,
	}
}

func ConvertPortfolio(dbPortfolio dbModel.Portfolio) model.Portfolio {
	return model.Portfolio{
		PortfolioID:   dbPortfolio.PortfolioID,
		PortfolioName: dbPortfolio.Name,
	}
}

func ConvertStockRemaining(stockRemaining dbModel.StockRemaining) model.StockRemaining {
	return model.StockRemaining{
		RowID: stockRemaining.RowID,
		PortfolioID: stockRemaining.PortfolioID,
		Ticker: stockRemaining.Ticker,
		Quantity: stockRemaining.Quantity,
		Price: stockRemaining.Price,
		DtCreate: stockRemaining.DtCreate,
		DtUpdate: stockRemaining.DtUpdate,
	}
}