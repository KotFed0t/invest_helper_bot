package telebotConverter

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/KotFed0t/invest_helper_bot/internal/model"
	"github.com/KotFed0t/invest_helper_bot/internal/model/moexModel"
	"github.com/KotFed0t/invest_helper_bot/internal/model/tg/tgCallback.go"
	"github.com/shopspring/decimal"
	tele "gopkg.in/telebot.v4"
)

func PortfolioDetailsResponse(portfolio model.PortfolioPage, stocksPerPage int) (text string, markup *tele.ReplyMarkup) {
	markup = &tele.ReplyMarkup{}
	var sb strings.Builder

	// Заголовок портфеля
	sb.WriteString(fmt.Sprintf("📊 Портфель: %s\n\n", portfolio.PortfolioName))
	sb.WriteString("💰 Балансы: \n")
	sb.WriteString(fmt.Sprintf("▸ в индексе: %s ₽\n", portfolio.BalanceInsideIndex.StringFixed(2)))
	sb.WriteString(fmt.Sprintf("▸ вне индекса: %s ₽\n\n", portfolio.BalanceOutsideIndex.StringFixed(2)))
	sb.WriteString(fmt.Sprintf("⚖️ Текущий вес %s %%\n", portfolio.TotalWeight.StringFixed(2)))
	sb.WriteString(fmt.Sprintf("🔀 Отклонение от индекса %s %%\n\n", portfolio.IndexOffset.StringFixed(2)))

	// Состав портфеля
	sb.WriteString("📋 Состав портфеля:\n\n")
	stockBtns := make([]tele.Btn, 0, len(portfolio.Stocks))
	for i, stock := range portfolio.Stocks {
		// Эмодзи с порядковым номером
		ordinal := fmt.Sprintf("%d)", i+1+(stocksPerPage*(portfolio.CurPage-1)))

		stockBtns = append(stockBtns, markup.Data(stock.Ticker, tgCallback.EditStockPrefix+stock.Ticker))

		sb.WriteString(fmt.Sprintf("%s %s (%s)\n", ordinal, stock.Ticker, stock.Shortname))
		sb.WriteString(fmt.Sprintf("▸ Вес: %s %%\n", stock.ActualWeight.StringFixed(2)))
		sb.WriteString(fmt.Sprintf("▸ целевой вес: %s %%\n", stock.TargetWeight.StringFixed(2)))
		sb.WriteString(fmt.Sprintf("▸ Кол-во: %d шт.\n", stock.Quantity))
		sb.WriteString(fmt.Sprintf("▸ Цена акции: %s ₽\n", stock.Price.StringFixed(2)))
		sb.WriteString(fmt.Sprintf("▸ Стоимость: %s ₽\n", stock.TotalPrice.StringFixed(2)))
		sb.WriteString(fmt.Sprintf("▸ Размер лота: %d\n", stock.Lotsize))
		sb.WriteString(fmt.Sprintf("▸ Цена лота: %s ₽\n\n", stock.Price.Mul(decimal.NewFromInt(int64(stock.Lotsize))).StringFixed(2)))
	}

	paginationBtns := make([]tele.Btn, 0, 3)
	if portfolio.CurPage > 1 {
		paginationBtns = append(paginationBtns, markup.Data("назад", tgCallback.ToPortfolioPage+strconv.Itoa((portfolio.CurPage-1))))
	}

	if portfolio.CurPage > 1 || portfolio.TotalPages > portfolio.CurPage {
		paginationBtns = append(paginationBtns, markup.Data(fmt.Sprintf("страница %d из %d", portfolio.CurPage, portfolio.TotalPages), tgCallback.PageNumber))
	}

	if portfolio.TotalPages > portfolio.CurPage {
		paginationBtns = append(paginationBtns, markup.Data("вперед", tgCallback.ToPortfolioPage+strconv.Itoa((portfolio.CurPage+1))))
	}

	addStockBtn := markup.Data("✚ Добавить акцию", tgCallback.AddStock)

	var calculatePurchaseBtn tele.Btn
	if portfolio.StocksCount > portfolio.StocksOutsideIndexCnt {
		calculatePurchaseBtn = markup.Data("Рассчитать закуп", tgCallback.CalculatePurchase)
	}

	var rebalanceWeights tele.Btn
	if !portfolio.TotalWeight.IsZero() && (portfolio.TotalWeight.LessThan(decimal.NewFromInt(99)) || portfolio.TotalWeight.GreaterThan(decimal.NewFromInt(101))) {
		rebalanceWeights = markup.Data("выровнять веса", tgCallback.RebalanceWeights)
	}

	var deletePortfolio tele.Btn
	if portfolio.BalanceInsideIndex.IsZero() && portfolio.BalanceOutsideIndex.IsZero() {
		deletePortfolio = markup.Data("⚠️ удалить портфель", tgCallback.InitDeletePortfolio)
	}

	backToPortfolioListBtn := markup.Data("К списку портфелей", tgCallback.BackToPortolioList)

	markup.Inline(
		markup.Row(addStockBtn, calculatePurchaseBtn),
		markup.Row(rebalanceWeights),
		markup.Row(stockBtns...),
		markup.Row(paginationBtns...),
		markup.Row(deletePortfolio),
		markup.Row(backToPortfolioListBtn),
	)

	return sb.String(), markup
}

func StockNotFoundMarkup() (markup *tele.ReplyMarkup) {
	markup = &tele.ReplyMarkup{}
	backToPortfolioBtn := markup.Data("назад к портфелю", tgCallback.BackToPortolio)
	addStockBtn := markup.Data("ввести другой тикер", tgCallback.AddStock)
	markup.Inline(
		markup.Row(addStockBtn),
		markup.Row(backToPortfolioBtn),
	)
	return markup
}

func StockDetailResponse(stock model.Stock, stockChanges *model.StockChanges) (text string, markup *tele.ReplyMarkup) {
	markup = &tele.ReplyMarkup{}
	sb := strings.Builder{}

	sb.WriteString(fmt.Sprintf("%s (%s)\n", stock.Ticker, stock.Shortname))
	sb.WriteString(fmt.Sprintf("▸ Вес: %s %%\n", stock.ActualWeight.StringFixed(2)))
	sb.WriteString(fmt.Sprintf("▸ Целевой вес: %s %%\n", stock.TargetWeight.StringFixed(2)))
	sb.WriteString(fmt.Sprintf("▸ Кол-во: %d шт.\n", stock.Quantity))
	sb.WriteString(fmt.Sprintf("▸ Цена акции: %s ₽\n", stock.Price.StringFixed(2)))
	sb.WriteString(fmt.Sprintf("▸ Стоимость: %s ₽\n", stock.TotalPrice.StringFixed(2)))
	sb.WriteString(fmt.Sprintf("▸ Размер лота: %d\n", stock.Lotsize))
	sb.WriteString(fmt.Sprintf("▸ Цена лота: %s ₽\n", stock.Price.Mul(decimal.NewFromInt(int64(stock.Lotsize))).StringFixed(2)))

	row1 := make([]tele.Btn, 0, 2)

	if stock.Quantity > 0 {
		sellStockBtn := markup.Data("продать", tgCallback.SellStock)
		row1 = append(row1, sellStockBtn)
	}

	buyStockBtn := markup.Data("купить", tgCallback.BuyStock)
	row1 = append(row1, buyStockBtn)

	changeWeightStockBtn := markup.Data("изменить вес", tgCallback.ChangeWeight)

	var deleteStockBtn tele.Btn
	if stock.Quantity == 0 {
		deleteStockBtn = markup.Data("⚠️ удалить из портфеля", tgCallback.DeleteStock)
	}

	var changePriceBtn tele.Btn
	var saveBtn tele.Btn

	if stockChanges != nil {
		sb.WriteString("\nИзменения:\n")
		if stockChanges.NewTargetWeight != nil {
			sb.WriteString(fmt.Sprintf("▸ Новый целевой вес: %s %%\n", stockChanges.NewTargetWeight.StringFixed(2)))
		}

		if stockChanges.Quantity != nil {
			var operation string
			if *stockChanges.Quantity < 0 {
				operation = "продажи"
			} else {
				operation = "покупки"
			}

			changePriceBtn = markup.Data(fmt.Sprintf("изменить цену %s", operation), tgCallback.ChangePrice)

			if *stockChanges.Quantity > 0 {
				sb.WriteString(fmt.Sprintf("▸ Акций к покупке: %d шт.\n", *stockChanges.Quantity))
			}

			if *stockChanges.Quantity < 0 {
				sb.WriteString(fmt.Sprintf("▸ Акций к продаже: %d шт.\n", *stockChanges.Quantity*-1))
			}

			var stockPrice, totalSum string

			if stockChanges.CustomPrice != nil {
				stockPrice = stockChanges.CustomPrice.StringFixed(2)

				totalSum = stockChanges.CustomPrice.
					Mul(decimal.NewFromInt(int64(*stockChanges.Quantity))).
					Abs().
					StringFixed(2)
			} else {
				stockPrice = stock.Price.StringFixed(2)

				totalSum = stock.Price.
					Mul(decimal.NewFromInt(int64(*stockChanges.Quantity))).
					Abs().
					StringFixed(2)
			}
			sb.WriteString(fmt.Sprintf("▸ Цена за акцию: %s ₽\n", stockPrice))
			sb.WriteString(fmt.Sprintf("▸ Сумма %s: %s ₽\n", operation, totalSum))
		}

		saveBtn = markup.Data("сохранить изменения", tgCallback.SaveStockChanges)
	}

	backToPortfolioBtn := markup.Data("назад к портфелю", tgCallback.BackToPortolio)

	markup.Inline(
		row1,
		markup.Row(changePriceBtn),
		markup.Row(changeWeightStockBtn),
		markup.Row(deleteStockBtn),
		markup.Row(backToPortfolioBtn),
		markup.Row(saveBtn),
	)

	return sb.String(), markup
}

func StockAddResponse(stock moexModel.StockInfo) (text string, markup *tele.ReplyMarkup) {
	markup = &tele.ReplyMarkup{}
	sb := strings.Builder{}

	sb.WriteString(fmt.Sprintf("%s (%s)\n", stock.Ticker, stock.Shortname))
	sb.WriteString(fmt.Sprintf("▸ Цена акции: %s ₽\n", stock.Price.StringFixed(2)))
	sb.WriteString(fmt.Sprintf("▸ Размер лота: %d\n", stock.Lotsize))
	sb.WriteString(fmt.Sprintf("▸ Цена лота: %s ₽\n", stock.Price.Mul(decimal.NewFromInt(int64(stock.Lotsize))).StringFixed(2)))

	addToPortfolioBtn := markup.Data("добавить в портфель", tgCallback.AddStockToPortfolio)

	addAnotherStockBtn := markup.Data("ввести другой тикер", tgCallback.AddStock)

	backToPortfolioBtn := markup.Data("назад к портфелю", tgCallback.BackToPortolio)

	markup.Inline(
		markup.Row(addToPortfolioBtn),
		markup.Row(addAnotherStockBtn),
		markup.Row(backToPortfolioBtn),
	)

	return sb.String(), markup
}

func CalculatedStockPurchaseResponse(stocks []model.StockPurchase, purchaseSum decimal.Decimal) (texts []string, markup *tele.ReplyMarkup) {
	markup = &tele.ReplyMarkup{}
	sb := strings.Builder{}
	actualPurchaseSum := decimal.NewFromInt(0)

	backToPortfolioBtn := markup.Data("назад к портфелю", tgCallback.BackToPortolio)
	markup.Inline(
		markup.Row(backToPortfolioBtn),
	)

	for i, stock := range stocks {
		ordinal := fmt.Sprintf("%d)", i+1)
		sb.WriteString(fmt.Sprintf("%s %s (%s)\n", ordinal, stock.Ticker, stock.Shortname))
		sb.WriteString(fmt.Sprintf("▸ лотов: %d шт\n", stock.LotsQuantity.IntPart()))
		sb.WriteString(fmt.Sprintf("▸ акций: %d шт\n", int64(stock.LotSize) * stock.LotsQuantity.IntPart()))

		sum := stock.StockPrice.Mul(decimal.NewFromInt(stock.LotsQuantity.IntPart() * int64(stock.LotSize)))
		actualPurchaseSum = actualPurchaseSum.Add(sum)
		sb.WriteString(fmt.Sprintf("▸ на сумму: %s ₽\n\n", sum.StringFixed(2)))

		if (i+1)%50 == 0 {
			texts = append(texts, sb.String())
			sb = strings.Builder{}
		}
	}

	sb.WriteString("Итоги:\n")
	sb.WriteString(fmt.Sprintf("▸ Сумма докупки: %s ₽\n", actualPurchaseSum.StringFixed(2)))
	sb.WriteString(fmt.Sprintf("▸ Остаток: %s ₽\n", purchaseSum.Sub(actualPurchaseSum).StringFixed(2)))

	texts = append(texts, sb.String())
	return texts, markup
}

func PortfolioListResponse(portfolios []model.Portfolio, portfoliosPerPage, curPage int, hasNextPage bool) (texts string, markup *tele.ReplyMarkup) {
	markup = &tele.ReplyMarkup{}
	sb := strings.Builder{}

	if len(portfolios) == 0 {
		return "список портфелей пуст", markup
	}

	portfolioBtnsRows := 0
	if len(portfolios)%5 == 0 {
		portfolioBtnsRows = len(portfolios) / 5
	} else {
		portfolioBtnsRows = len(portfolios)/5 + 1
	}

	menuRows := make([]tele.Row, 0, portfolioBtnsRows+1)

	sb.WriteString("Список ваших портфелей:\n\n")
	for i, portfolio := range portfolios {
		if i%5 == 0 {
			menuRows = append(menuRows, make(tele.Row, 0, 5))
		}
		ordinal := fmt.Sprintf("%d)", i+1+(portfoliosPerPage*(curPage-1)))
		sb.WriteString(fmt.Sprintf("%s %s\n\n", ordinal, portfolio.PortfolioName))
		btn := markup.Data(portfolio.PortfolioName, tgCallback.EditPortfolioPrefix+strconv.FormatInt(portfolio.PortfolioID, 10))
		menuRows[len(menuRows)-1] = append(menuRows[len(menuRows)-1], btn)
	}

	paginationBtns := make([]tele.Btn, 0)
	if curPage > 1 {
		paginationBtns = append(paginationBtns, markup.Data("назад", tgCallback.ToPortfolioListPage+strconv.Itoa((curPage-1))))
	}

	if curPage > 1 || hasNextPage {
		paginationBtns = append(paginationBtns, markup.Data(fmt.Sprintf("страница %d", curPage), tgCallback.PageNumber))
	}

	if hasNextPage {
		paginationBtns = append(paginationBtns, markup.Data("вперед", tgCallback.ToPortfolioListPage+strconv.Itoa((curPage+1))))
	}

	generateReportBtn := markup.Data("сгенерировать отчет", tgCallback.GenerateReport)
	
	menuRows = append(menuRows, markup.Row(generateReportBtn), markup.Row(paginationBtns...))

	markup.Inline(menuRows...)

	return sb.String(), markup
}

func DeletePortfolioConfirmation() (markup *tele.ReplyMarkup) {
	markup = &tele.ReplyMarkup{}
	backToPortfolioBtn := markup.Data("назад к портфелю", tgCallback.BackToPortolio)
	deletePortfolioBtn := markup.Data("подтвердить удаление", tgCallback.ProcessDeletePortfolio)
	markup.Inline(
		markup.Row(backToPortfolioBtn),
		markup.Row(deletePortfolioBtn),
	)
	return markup
}
