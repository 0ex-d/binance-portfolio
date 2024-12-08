package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	log "github.com/sirupsen/logrus"
)

type Template struct {
	tmpls *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.tmpls.ExecuteTemplate(w, name, data)
}

type HeaderTr struct {
	Name  string
	Icon  string
	Class string
}

type TableSection struct {
	Header []HeaderTr
}

type IndexPage struct {
	Email        string
	ErrorMsgs    map[string]string
	TableSection TableSection
}

var ErrorGenericResp = errors.New("error fetching data or pair doesn't exist for this user")

func getApiAndSecretKeys() (string, string) {
	apiKey := os.Getenv("BINANCE_API_KEY")
	secretKey := os.Getenv("BINANCE_SECRET_KEY")
	if apiKey == "" {
		log.Fatal("You obviously didn't read the Readme.md! :( BINANCE_API_KEY and BINANCE_SECRET_KEY are required!")
	}
	return apiKey, secretKey
}

func getTs() string {
	return strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10)
}

type Prices []float64

type stas[T float64 | int] interface{}
type RESTResp[T interface{} | map[string]interface{}] struct {
	Data T
	Err  interface{}
}

type PortfolioBalance struct {
	Symbol             string  `json:"symbol"`
	QuoteSymbol        string  `json:"quote_symbol"`
	Free               float64 `json:"free"`
	Locked             float64 `json:"locked"`
	Price              float64 `json:"price"`
	PriceFlag          string  `json:"price_flag"`
	PriceChangeValue   float64 `json:"price_change_value"`
	PriceChangePercent float64 `json:"price_change_percent"`
	QuoteValue         float64 `json:"quote_value"`
}

func signParams(message, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

var portfolioBalancesInMemory []PortfolioBalance

func calculateRealizedPNL(trades []Trade, avgBuyPrice float64) (float64, error) {
	var realizedPNL float64
	for _, trade := range trades {
		tradePrice, _ := strconv.ParseFloat(trade.Price, 64)
		tradeQty, _ := strconv.ParseFloat(trade.Qty, 64)
		if trade.IsBuyer {
			continue
		}
		realizedPNL += (tradePrice - avgBuyPrice) * tradeQty
	}
	return realizedPNL, nil
}

func getPortfolioBalancesAndCCData(currency string) ([]PortfolioBalance, error) {
	var err error
	var balances []Balance
	var portfolioBalances []PortfolioBalance
	if len(portfolioBalancesInMemory) != 0 {
		log.Info("[getPortfolioBalancesAndCCData]: Getting from memory")
		return portfolioBalancesInMemory, err
	}

	balances, err = GetAccountBalances()
	if err != nil {
		return portfolioBalances, err
	}

	var ccDataInstruments []string
	for _, balance := range balances {
		if balance.Asset == "USDT" || balance.Asset == "GBP" || balance.Asset == "USD" || balance.Asset == currency {
			continue
		}
		ccDataInstruments = append(ccDataInstruments, fmt.Sprintf("%s-%s", balance.Asset, currency))
	}
	spotResponse, err := GetCCDataCurrentTickerPrice(strings.Join(ccDataInstruments, ","), os.Getenv("CC_API_KEY"))
	if err != nil {
		return portfolioBalances, err
	}

	for _, balance := range balances {
		if balance.Asset == "USDT" || balance.Asset == "GBP" || balance.Asset == "USD" || balance.Asset == currency {
			portfolioBalances = append(portfolioBalances, PortfolioBalance{
				Symbol: balance.Asset,
				Free:   balance.Free,
			})
			continue
		}
		instrument := fmt.Sprintf("%s-%s", balance.Asset, currency)
		currentInstrument := spotResponse.Data[instrument]
		assetValue := balance.Free * currentInstrument.Price
		portfolioBalances = append(portfolioBalances, PortfolioBalance{
			Symbol:             balance.Asset,
			QuoteSymbol:        currency,
			Free:               balance.Free,
			QuoteValue:         assetValue,
			Price:              currentInstrument.Price,
			PriceFlag:          currentInstrument.PriceFlag,
			PriceChangeValue:   currentInstrument.CurrentDayChange,
			PriceChangePercent: currentInstrument.CurrentDayChangePercentage,
		})
	}
	sort.Slice(portfolioBalances, func(i, j int) bool {
		return portfolioBalances[i].Free > portfolioBalances[j].Free
	})
	portfolioBalancesInMemory = portfolioBalances

	return portfolioBalances, nil
}

func getTotalPortfolioValue(currency string) (float64, error) {
	balances, err := GetAccountBalances()
	if err != nil {
		return 0, err
	}

	var totalValue float64
	var instruments []string
	for _, balance := range balances {
		if balance.Asset == "USDT" || balance.Asset == "GBP" || balance.Asset == "USD" {
			continue
		}
		instruments = append(instruments, fmt.Sprintf("%s-%s", balance.Asset, currency))
	}
	spotResponse, err := GetCCDataCurrentTickerPrice(strings.Join(instruments, ","), os.Getenv("CC_API_KEY"))
	if err != nil {
		return 0, err
	}
	for _, balance := range balances {
		if balance.Asset == "USDT" || balance.Asset == "GBP" || balance.Asset == "USD" {
			continue
		}
		instrument := fmt.Sprintf("%s-%s", balance.Asset, currency)
		currentInstrument := spotResponse.Data[instrument]
		assetValue := balance.Free * currentInstrument.Price
		totalValue += assetValue
	}
	return totalValue, nil
}

func main() {
	var err error
	envFile := ".env"
	err = godotenv.Load(envFile)
	if err != nil {
		log.Fatal(fmt.Sprintf("You must provide an %s file to continue - ", envFile), err)
	}

	e := echo.New()

	e.Renderer = &Template{
		tmpls: template.Must(template.ParseGlob("views/*.html")),
	}

	e.Use(middleware.Logger())
	e.Static("/src", "src")

	e.GET("/orders", func(c echo.Context) error {
		symbol := c.QueryParam("symbol")
		limit := c.QueryParam("limit")
		if strings.TrimSpace(symbol) == "" {
			return c.JSON(400, map[string]interface{}{"Err": "strange input", "Data": nil})
		}
		if strings.TrimSpace(limit) == "" {
			limit = "1000"
		}
		var data []Order
		data, err = GetAllOrders(symbol, limit)
		return c.JSON(200, data)
	})

	e.GET("/trades", func(c echo.Context) error {
		symbol := c.QueryParam("symbol")
		limit := c.QueryParam("limit")
		if strings.TrimSpace(symbol) == "" {
			return c.JSON(400, map[string]interface{}{"Err": "strange input", "Data": nil})
		}
		if strings.TrimSpace(limit) == "" {
			limit = "1000"
		}
		var data []Trade
		data, err = GetTradesList(symbol, limit)
		return c.JSON(200, data)
	})

	e.GET("/wallet", func(c echo.Context) error {
		var balances []PortfolioBalance
		currency := "USDT"
		balances, err := getPortfolioBalancesAndCCData(currency)
		if err != nil {
			return c.JSON(400, RESTResp[[]PortfolioBalance]{Data: balances, Err: errors.New("error getting balances")})
		}
		return c.JSON(200, RESTResp[[]PortfolioBalance]{Data: balances})
	})

	e.GET("/", func(c echo.Context) error {
		headerTrs := []HeaderTr{{
			Name:  "Asset",
			Icon:  "fa fa-caret-up",
			Class: "",
		},
			{
				Name:  "Price",
				Icon:  "fa fa-caret-up",
				Class: "",
			},
			{
				Name:  "Total Value(TV)",
				Icon:  "fa fa-caret-up",
				Class: "",
			},
			{
				Name:  "PnL",
				Icon:  "fa fa-caret-up",
				Class: "",
			},
			{
				Name:  "Change",
				Icon:  "fa fa-caret-up",
				Class: "",
			},
		}
		tSection := TableSection{Header: headerTrs}
		return c.Render(200, "index", IndexPage{
			TableSection: tSection,
		})
	})
	port := "42000"
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", port)))
}
