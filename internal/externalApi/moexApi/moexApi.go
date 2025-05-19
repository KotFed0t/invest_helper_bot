package moexApi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/KotFed0t/invest_helper_bot/config"
	"github.com/KotFed0t/invest_helper_bot/internal/externalApi"
	"github.com/KotFed0t/invest_helper_bot/internal/model/moexModel"
	"github.com/KotFed0t/invest_helper_bot/utils"
	"github.com/go-resty/resty/v2"
	"github.com/shopspring/decimal"
)

type MoexApi struct {
	client *resty.Client
}

func New(cfg *config.Config) *MoexApi {
	client := resty.New().
		SetDebug(cfg.API.Debug).
		SetTimeout(cfg.API.Timeout).
		SetBaseURL(cfg.API.MoexApi.Url)
	return &MoexApi{client: client}
}

func (a *MoexApi) getStocsInfo(ctx context.Context, tickers ...string) (moexModel.RawStocksInfo, error) {
	rqId := utils.GetRequestIDFromCtx(ctx)
	url := "/iss/engines/stock/markets/shares/boards/TQBR/securities.json"
	params := map[string]string{
		"iss.meta":           "off",
		"securities.columns": "SECID,SHORTNAME,LOTSIZE,CURRENCYID,STATUS",
		"marketdata.columns": "SECID,LAST",
	}

	if len(tickers) > 0 {
		params["securities"] = strings.Join(tickers, ",")
	}

	slog.Debug("start MoexApi.getStocsInfo request", slog.String("rqID", rqId), slog.String("url", url), slog.Any("params", params))

	resp, err := a.client.R().
		SetHeader("Accept", "application/json").
		SetQueryParams(params).
		Get(url)

	if err != nil {
		slog.Error("error while dialing MoexApi", slog.String("err", err.Error()), slog.String("rqID", rqId))
		return moexModel.RawStocksInfo{}, err
	}

	rawStocksInfo := moexModel.RawStocksInfo{}
	err = json.Unmarshal(resp.Body(), &rawStocksInfo)
	if err != nil {
		slog.Error("can't unmarshall response into moexModel.RawStocksInfo", slog.String("err", err.Error()), slog.String("rqID", rqId))
		return moexModel.RawStocksInfo{}, err
	}

	slog.Debug("MoexApi.getStocsInfo request complete", slog.String("rqID", rqId))

	return rawStocksInfo, nil
}

func (a *MoexApi) GetAllStocsInfo(ctx context.Context) ([]moexModel.StockInfo, error) {
	rqId := utils.GetRequestIDFromCtx(ctx)

	slog.Debug("start MoexApi.GetStocsInfo request", slog.String("rqID", rqId))

	rawStocksInfo, err := a.getStocsInfo(ctx)
	if err != nil {
		slog.Error("got error from MoexApi.getStocsInfo", slog.String("err", err.Error()), slog.String("rqID", rqId))
		return nil, err
	}

	res, err := a.parseRawStocksInfoToSlice(rawStocksInfo)
	if err != nil {
		slog.Error("can't parse raw data", slog.String("err", err.Error()), slog.String("rqID", rqId))
		return nil, err
	}

	slog.Debug("MoexApi.GetStocsInfo request complete", slog.String("rqID", rqId))

	return res, nil
}

func (a *MoexApi) GetStocInfo(ctx context.Context, ticker string) (moexModel.StockInfo, error) {
	rqId := utils.GetRequestIDFromCtx(ctx)

	slog.Debug("start MoexApi.GetStocInfo request", slog.String("rqID", rqId))

	rawStocksInfo, err := a.getStocsInfo(ctx, ticker)
	if err != nil {
		slog.Error("got error from MoexApi.getStocsInfo", slog.String("err", err.Error()), slog.String("rqID", rqId))
		return moexModel.StockInfo{}, err
	}

	res, err := a.parseRawStocksInfoSingle(rawStocksInfo)
	if err != nil {
		slog.Error("can't parse raw data", slog.String("err", err.Error()), slog.String("rqID", rqId))
		return moexModel.StockInfo{}, err
	}

	slog.Debug("MoexApi.GetStocInfo request complete", slog.String("rqID", rqId))

	return res, nil
}

func (a *MoexApi) GetStocsInfo(ctx context.Context, tickers []string) (map[string]moexModel.StockInfo, error) {
	rqId := utils.GetRequestIDFromCtx(ctx)
	
	slog.Debug("start MoexApi.GetStocsInfo request", slog.String("rqID", rqId))

	rawStocksInfo, err := a.getStocsInfo(ctx, tickers...)
	if err != nil {
		slog.Error("got error from MoexApi.getStocsInfo", slog.String("err", err.Error()), slog.String("rqID", rqId))
		return nil, err
	}

	stocksInfoMap, err := a.parseRawStocksInfoToMap(rawStocksInfo)
	if err != nil {
		slog.Error("can't parse raw data to map", slog.String("err", err.Error()), slog.String("rqID", rqId))
		return nil, err
	}

	slog.Debug("MoexApi.GetStocsInfo request complete", slog.String("rqID", rqId))

	return stocksInfoMap, nil
}

func (a *MoexApi) parseRawStocksInfoToSlice(rawStocksInfo moexModel.RawStocksInfo) ([]moexModel.StockInfo, error) {
	if len(rawStocksInfo.Marketdata.Data) != len(rawStocksInfo.Securities.Data) {
		return nil, errors.New("lengths Marketdata != Securities")
	}

	res := make([]moexModel.StockInfo, 0, len(rawStocksInfo.Marketdata.Data))

	err := a.handleRawStocksInfo(rawStocksInfo, func(stock moexModel.StockInfo){
		res = append(res, stock)
	})
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (a *MoexApi) parseRawStocksInfoToMap(rawStocksInfo moexModel.RawStocksInfo) (map[string]moexModel.StockInfo, error) {
	if len(rawStocksInfo.Marketdata.Data) != len(rawStocksInfo.Securities.Data) {
		return nil, errors.New("lengths Marketdata != Securities")
	}

	res := make(map[string]moexModel.StockInfo, len(rawStocksInfo.Marketdata.Data))

	err := a.handleRawStocksInfo(rawStocksInfo, func(stock moexModel.StockInfo){
		res[stock.Ticker] = stock
	})
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (a *MoexApi) parseRawStocksInfoSingle(rawStocksInfo moexModel.RawStocksInfo) (moexModel.StockInfo, error) {
	if len(rawStocksInfo.Marketdata.Data) != len(rawStocksInfo.Securities.Data) {
		return moexModel.StockInfo{}, errors.New("lengths Marketdata != Securities")
	}

	if len(rawStocksInfo.Marketdata.Data) == 0 {
		return moexModel.StockInfo{}, externalApi.ErrNotFound
	} 

	res, err := a.parseRawStocksInfoToSlice(rawStocksInfo)
	if err != nil {
		return moexModel.StockInfo{}, err
	}

	if len(res) != 1 {
		return moexModel.StockInfo{}, errors.New("unexpected slice lenght, expected only 1 element")
	}

	return res[0], nil
}

func (a *MoexApi) handleRawStocksInfo(rawStocksInfo moexModel.RawStocksInfo, handleFn func(stock moexModel.StockInfo)) error {
	if len(rawStocksInfo.Marketdata.Data) != len(rawStocksInfo.Securities.Data) {
		return errors.New("lengths Marketdata != Securities")
	}

	for i := 0; i < len(rawStocksInfo.Marketdata.Data); i++ {
		if len(rawStocksInfo.Marketdata.Data[i]) != len(rawStocksInfo.Marketdata.Columns) {
			return errors.New("invalid Marketdata")
		}

		if len(rawStocksInfo.Securities.Data[i]) != len(rawStocksInfo.Securities.Columns) {
			return errors.New("invalid Securities")
		}

		stockInfo := moexModel.StockInfo{}

		for j := 0; j < len(rawStocksInfo.Marketdata.Columns); j++ {
			ok := true
			switch rawStocksInfo.Marketdata.Columns[j] {
			case "SECID":
				stockInfo.Ticker, ok = rawStocksInfo.Marketdata.Data[i][j].(string)
			case "LAST":
				if rawStocksInfo.Marketdata.Data[i][j] != nil {
					var price float64
					price, ok = rawStocksInfo.Marketdata.Data[i][j].(float64)
					if ok {
						stockInfo.Price = decimal.NewFromFloat(price)
					}
				}
			default:
				return fmt.Errorf("unknown column %s", rawStocksInfo.Marketdata.Columns[j])
			}

			if !ok {
				return fmt.Errorf("invalid type %s = %v", rawStocksInfo.Marketdata.Columns[j], rawStocksInfo.Marketdata.Data[i][j])
			}
		}

		for j := 0; j < len(rawStocksInfo.Securities.Columns); j++ {
			ok := true
			switch rawStocksInfo.Securities.Columns[j] {
			case "SECID":
				if rawStocksInfo.Securities.Data[i][j] != stockInfo.Ticker {
					return fmt.Errorf("secID in securities and market data is not equal %s and %s", rawStocksInfo.Securities.Data[i][j], stockInfo.Ticker)
				}
			case "SHORTNAME":
				stockInfo.Shortname, ok = rawStocksInfo.Securities.Data[i][j].(string)
			case "LOTSIZE":
				var f float64
				f, ok = rawStocksInfo.Securities.Data[i][j].(float64)
				if ok {
					stockInfo.Lotsize = int(f)
				}
			case "CURRENCYID":
				stockInfo.CurrencyID, ok = rawStocksInfo.Securities.Data[i][j].(string)
				if ok && stockInfo.CurrencyID == "SUR" {
					stockInfo.CurrencyID = "RUB"
				}
			case "STATUS":
				var status string // чтобы далее не затенить переменную ok
				status, ok = rawStocksInfo.Securities.Data[i][j].(string)
				if ok && status == "A" {
					stockInfo.Status = true
				}
			default:
				return fmt.Errorf("unknownd column %s", rawStocksInfo.Securities.Columns[j])
			}

			if !ok {
				return fmt.Errorf("invalid type %s = %v", rawStocksInfo.Securities.Columns[j], rawStocksInfo.Securities.Data[i][j])
			}
		}
		handleFn(stockInfo)
	}
	return nil
}
