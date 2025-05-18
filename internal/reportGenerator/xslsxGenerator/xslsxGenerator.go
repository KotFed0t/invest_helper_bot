package xslsxGenerator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/KotFed0t/invest_helper_bot/internal/model"
	"github.com/KotFed0t/invest_helper_bot/utils"
	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
)

type XSLSXGenerator struct{}

func New() *XSLSXGenerator {
	return &XSLSXGenerator{}
}

func (g *XSLSXGenerator) Generate(ctx context.Context, portfolios []model.PortfolioFullInfo) (fileBytes []byte, fileExtension string, err error) {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "XSLSXGenerator.Generate"

	if len(portfolios) == 0 {
		return nil, "", errors.New("empty portfolios")
	}

	slog.Debug("Generate start", slog.String("rqID", rqID), slog.String("op", op))

	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			slog.Error("got error while closing file", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		}
	}()

	for i, portfolio := range portfolios {
		err := g.fillSheet(ctx, f, portfolio, i+1)
		if err != nil {
			return nil, "", err
		}
	}

	// Удаляем лист по умолчанию "Sheet1"
	if err := f.DeleteSheet("Sheet1"); err != nil {
		slog.Error("got error while deleting Sheet1", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		slog.Error("got error while Saving file to bytes buffer", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return nil, "", err
	}

	slog.Debug("Generate completed", slog.String("rqID", rqID), slog.String("op", op))

	return buf.Bytes(), ".xlsx", nil
}

func (g *XSLSXGenerator) fillSheet(ctx context.Context, f *excelize.File, portfolio model.PortfolioFullInfo, ordinal int) error {
	rqID := utils.GetRequestIDFromCtx(ctx)
	op := "XSLSXGenerator.fillSheet"

	sheetName := fmt.Sprintf("%d. %s", ordinal, portfolio.PortfolioName)
	_, err := f.NewSheet(sheetName)
	if err != nil {
		slog.Error("got error while creating NewSheet", slog.String("rqID", rqID), slog.String("op", op), slog.String("err", err.Error()))
		return err
	}

	// котировки
	err = f.MergeCell(sheetName, "A1", "E1")
	if err != nil {
		return err
	}

	f.SetCellValue(sheetName, "A1", "Котировки")

	styleID, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Font: &excelize.Font{
			Bold: true,
			Size: 11,
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Pattern: 1,
			Color:   []string{"#cfe2f3"}, // Светло-голубой цвет
		},
	})
	if err != nil {
		return err
	}

	if err := f.SetCellStyle(sheetName, "A1", "A1", styleID); err != nil {
		return fmt.Errorf("ошибка применения стиля: %w", err)
	}

	_ = f.SetCellStr(sheetName, "A2", "название")
	_ = f.SetCellStr(sheetName, "B2", "тикер")
	_ = f.SetCellStr(sheetName, "C2", "цена")
	_ = f.SetCellStr(sheetName, "D2", "лот")
	_ = f.SetCellStr(sheetName, "E2", "цена за лот")

	// в портфеле
	err = f.MergeCell(sheetName, "F1", "G1")
	if err != nil {
		return err
	}

	f.SetCellValue(sheetName, "F1", "В портфеле")

	styleID, err = f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Font: &excelize.Font{
			Bold: true,
			Size: 11,
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Pattern: 1,
			Color:   []string{"#d9ead3"}, // Светло-зеленый цвет
		},
	})
	if err != nil {
		return err
	}

	if err := f.SetCellStyle(sheetName, "F1", "F1", styleID); err != nil {
		return fmt.Errorf("ошибка применения стиля: %w", err)
	}

	_ = f.SetCellStr(sheetName, "F2", "кол-во акций")
	_ = f.SetCellStr(sheetName, "G2", "сумма")

	// веса
	err = f.MergeCell(sheetName, "H1", "I1")
	if err != nil {
		return err
	}

	f.SetCellValue(sheetName, "H1", "Веса")

	styleID, err = f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Font: &excelize.Font{
			Bold: true,
			Size: 11,
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Pattern: 1,
			Color:   []string{"#f9cb9c"}, // Светло-оранжевый цвет
		},
	})
	if err != nil {
		return err
	}

	if err := f.SetCellStyle(sheetName, "H1", "H1", styleID); err != nil {
		return fmt.Errorf("ошибка применения стиля: %w", err)
	}

	_ = f.SetCellStr(sheetName, "H2", "целевой")
	_ = f.SetCellStr(sheetName, "I2", "текущий")

	// отклонение от индекса
	err = f.MergeCell(sheetName, "J1", "K1")
	if err != nil {
		return err
	}

	f.SetCellValue(sheetName, "J1", "Отклонение от индекса")

	styleID, err = f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Font: &excelize.Font{
			Bold: true,
			Size: 11,
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Pattern: 1,
			Color:   []string{"#f4cccc"}, // Светло-розовый цвет
		},
	})
	if err != nil {
		return err
	}

	if err := f.SetCellStyle(sheetName, "J1", "J1", styleID); err != nil {
		return fmt.Errorf("ошибка применения стиля: %w", err)
	}

	_ = f.SetCellStr(sheetName, "J2", "процент")
	_ = f.SetCellStr(sheetName, "K2", "рубли")

	for i, stock := range portfolio.Stocks {
		_ = f.SetCellStr(sheetName, fmt.Sprintf("A%d", i+3), stock.Shortname)
		_ = f.SetCellStr(sheetName, fmt.Sprintf("B%d", i+3), stock.Ticker)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("C%d", i+3), stock.Price.InexactFloat64())
		_ = f.SetCellInt(sheetName, fmt.Sprintf("D%d", i+3), int64(stock.Lotsize))
		_ = f.SetCellValue(sheetName, fmt.Sprintf("E%d", i+3), stock.Price.Mul(decimal.NewFromInt(int64(stock.Lotsize))).InexactFloat64())

		_ = f.SetCellInt(sheetName, fmt.Sprintf("F%d", i+3), int64(stock.Quantity))
		_ = f.SetCellValue(sheetName, fmt.Sprintf("G%d", i+3), stock.TotalPrice.InexactFloat64())

		_ = f.SetCellValue(sheetName, fmt.Sprintf("H%d", i+3), stock.TargetWeight.InexactFloat64())
		_ = f.SetCellValue(sheetName, fmt.Sprintf("I%d", i+3), stock.ActualWeight.InexactFloat64())

		_ = f.SetCellValue(sheetName, fmt.Sprintf("J%d", i+3), stock.ActualWeight.Sub(stock.TargetWeight).InexactFloat64())

		totalPriceDelta := stock.TotalPrice.Sub(portfolio.BalanceInsideIndex.Mul(stock.TargetWeight.Div(decimal.NewFromInt(100))))
		_ = f.SetCellValue(sheetName, fmt.Sprintf("K%d", i+3), totalPriceDelta.InexactFloat64())
	}

	// история операций
	rowNum := len(portfolio.Stocks) + 6

	err = f.MergeCell(sheetName, fmt.Sprintf("A%d", rowNum), fmt.Sprintf("G%d", rowNum))
	if err != nil {
		return err
	}

	f.SetCellValue(sheetName, fmt.Sprintf("A%d", rowNum), "История операций")

	styleID, err = f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Font: &excelize.Font{
			Bold: true,
			Size: 11,
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Pattern: 1,
			Color:   []string{"#cccccc"}, // Серый цвет
		},
	})
	if err != nil {
		return err
	}

	if err := f.SetCellStyle(sheetName, fmt.Sprintf("A%d", rowNum), fmt.Sprintf("A%d", rowNum), styleID); err != nil {
		return fmt.Errorf("ошибка применения стиля: %w", err)
	}

	rowNum++
	_ = f.SetCellStr(sheetName, fmt.Sprintf("A%d", rowNum), "название")
	_ = f.SetCellStr(sheetName, fmt.Sprintf("B%d", rowNum), "тикер")
	_ = f.SetCellStr(sheetName, fmt.Sprintf("C%d", rowNum), "кол-во")
	_ = f.SetCellStr(sheetName, fmt.Sprintf("D%d", rowNum), "цена акции")
	_ = f.SetCellStr(sheetName, fmt.Sprintf("E%d", rowNum), "сумма покупки")
	_ = f.SetCellStr(sheetName, fmt.Sprintf("F%d", rowNum), "валюта")
	_ = f.SetCellStr(sheetName, fmt.Sprintf("G%d", rowNum), "дата")

	for _, operation := range portfolio.StockOperations {
		rowNum++
		_ = f.SetCellStr(sheetName, fmt.Sprintf("A%d", rowNum), operation.Shortname)
		_ = f.SetCellStr(sheetName, fmt.Sprintf("B%d", rowNum), operation.Ticker)
		_ = f.SetCellInt(sheetName, fmt.Sprintf("C%d", rowNum), int64(operation.Quantity))
		_ = f.SetCellValue(sheetName, fmt.Sprintf("D%d", rowNum), operation.Price.InexactFloat64())
		_ = f.SetCellValue(sheetName, fmt.Sprintf("E%d", rowNum), operation.TotalPrice.InexactFloat64())
		_ = f.SetCellStr(sheetName, fmt.Sprintf("F%d", rowNum), operation.Currency)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("G%d", rowNum), operation.DtCreate)
	}

	return nil
}
