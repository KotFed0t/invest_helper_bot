package telebotConverter

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/KotFed0t/invest_helper_bot/internal/model"
	"github.com/KotFed0t/invest_helper_bot/internal/model/tg/tgCallback.go"
	tele "gopkg.in/telebot.v4"
)

func PortfolioDetailsResponse(portfolio model.Portfolio) (text string, markup *tele.ReplyMarkup) {
	markup = &tele.ReplyMarkup{}
	var sb strings.Builder

	// Заголовок портфеля
	sb.WriteString(fmt.Sprintf("📊 Портфель: %s\n", portfolio.Name))
	sb.WriteString(fmt.Sprintf("💰 Баланс: %.0f ₽\n", portfolio.TotalBalance))
	sb.WriteString(fmt.Sprintf(" - Текущий вес %.1f%%\n\n", portfolio.TotalWeight))

	// Состав портфеля
	sb.WriteString("📋 Состав портфеля:\n\n")
	stockBtns := make([]tele.Btn, 0, len(portfolio.Stocks))
	for _, stock := range portfolio.Stocks {
		// Эмодзи с порядковым номером
		emoji := fmt.Sprintf("%d️⃣", stock.Ordinal)

		stockBtns = append(stockBtns, markup.Data(stock.Ticker, tgCallback.AddStock+stock.Ticker))

		sb.WriteString(fmt.Sprintf("%s **%s (%s)**\n", emoji, stock.Ticker, stock.Shortname))
		sb.WriteString(fmt.Sprintf("   ▸ Вес: **%.1f%%**\n", stock.ActualWeight))

		sb.WriteString(fmt.Sprintf("   ▸ Эталонный вес: %.1f%%\n", stock.TargetWeight))

		sb.WriteString(fmt.Sprintf("   ▸ Кол-во: **%d шт.**\n", stock.Quantity))

		sb.WriteString(fmt.Sprintf("   ▸ Цена акции: %.0f ₽\n", stock.Price))

		sb.WriteString(fmt.Sprintf("   ▸ Стоимость: **%.0f ₽**\n\n", stock.TotalPrice))
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
