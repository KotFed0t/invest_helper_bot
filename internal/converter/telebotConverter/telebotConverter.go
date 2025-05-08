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

func PortfolioDetailsResponse(portfolio model.Portfolio) (text string, markup *tele.ReplyMarkup) {
	markup = &tele.ReplyMarkup{}
	var sb strings.Builder

	// Заголовок портфеля
	sb.WriteString(fmt.Sprintf("📊 Портфель: %s\n", portfolio.Name))
	sb.WriteString(fmt.Sprintf("💰 Баланс: %s ₽\n", portfolio.TotalBalance.StringFixed(2)))
	sb.WriteString(fmt.Sprintf(" - Текущий вес %s\n", portfolio.TotalWeight.StringFixed(2)))

	// Состав портфеля
	sb.WriteString("📋 Состав портфеля:\n\n")
	stockBtns := make([]tele.Btn, 0, len(portfolio.Stocks))
	for _, stock := range portfolio.Stocks {
		// Эмодзи с порядковым номером
		emoji := fmt.Sprintf("%d️⃣", stock.Ordinal)

		stockBtns = append(stockBtns, markup.Data(stock.Ticker, tgCallback.AddStock+stock.Ticker))

		sb.WriteString(fmt.Sprintf("%s %s (%s)\n", emoji, stock.Ticker, stock.Shortname))
		sb.WriteString(fmt.Sprintf("▸ Вес: %s\n", stock.ActualWeight.StringFixed(2)))
		sb.WriteString(fmt.Sprintf("▸ целевой вес: %s\n", stock.TargetWeight.StringFixed(2)))
		sb.WriteString(fmt.Sprintf("▸ Кол-во: %d шт.\n", stock.Quantity))
		sb.WriteString(fmt.Sprintf("▸ Цена акции: %s ₽\n", stock.Price.StringFixed(2)))
		sb.WriteString(fmt.Sprintf("▸ Стоимость: %s ₽\n", stock.TotalPrice.StringFixed(2)))
		sb.WriteString(fmt.Sprintf("▸ Размер лота: %d\n", stock.Lotsize))
		sb.WriteString(fmt.Sprintf("▸ Цена лота: %s ₽\n", stock.Price.Mul(decimal.NewFromInt(int64(stock.Lotsize))).StringFixed(2)))
	}

	paginationBtns := make([]tele.Btn, 0, 2)
	if portfolio.CurPage > 0 {
		paginationBtns = append(paginationBtns, markup.Data("предыдущая", tgCallback.PrevPagePrefix+strconv.Itoa((portfolio.CurPage-1))))
	}

	if portfolio.HasNextPage {
		paginationBtns = append(paginationBtns, markup.Data("следующая", tgCallback.NextPagePrefix+strconv.Itoa((portfolio.CurPage+1))))
	}

	addStockBtn := markup.Data("➕ Добавить акцию", tgCallback.AddStock)
	markup.Inline(
		markup.Row(addStockBtn),
		markup.Row(stockBtns...),
		markup.Row(paginationBtns...),
	)

	return sb.String(), markup
}

func StockNotFoundMarkup() (markup *tele.ReplyMarkup) {
	markup = &tele.ReplyMarkup{}
	addStockBtn := markup.Data("ввести другой тикер", tgCallback.AddStock)
	markup.Inline(markup.Row(addStockBtn))
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

	deleteStockBtn := markup.Data("удалить из портфеля", "TODO")

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

	backToPortfolioBtn := markup.Data("назад к портфелю", "TODO")

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

	backToPortfolioBtn := markup.Data("назад к портфелю", "TODO")

	markup.Inline(
		markup.Row(addToPortfolioBtn),
		markup.Row(addAnotherStockBtn),
		markup.Row(backToPortfolioBtn),
	)

	return sb.String(), markup
}
