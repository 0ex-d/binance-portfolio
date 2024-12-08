package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sirupsen/logrus"
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

type stas[T float64 | int] interface {
}
type RESTResp struct {
	Data map[string]interface{}
	Err  interface{}
}

func signParams(message, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

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
	e.GET("/portfolio", func(c echo.Context) error {
		pair := c.QueryParam("pair")
		limit := c.QueryParam("limit")
		currency := c.QueryParam("currency")
		if strings.TrimSpace(pair) == "" {
			return c.JSON(400, map[string]interface{}{"Err": "strange input", "Data": nil})
		}
		if strings.TrimSpace(limit) == "" {
			limit = "1000"
		}
		if strings.TrimSpace(currency) == "" {
			currency = "USDT"
		}
		b := strings.Split(pair, "-")
		if len(b) != 2 {
			return c.JSON(400, map[string]interface{}{"Err": "strange input", "Data": nil})
		}
		asset := b[0]
		symbol := strings.Join(b, "")
		price, err := GetCurrentTickerPrice(symbol)
		if err != nil {
			logrus.Error("Error fetching current price:", err)
			return c.JSON(400, map[string]interface{}{"Err": ErrorGenericResp.Error(), "Data": nil})
		}
		balance, err := GetAccountBalance(asset)
		if err != nil {
			logrus.Error("Error fetching account balance:", err)
			return c.JSON(400, map[string]interface{}{"Err": ErrorGenericResp.Error(), "Data": nil})
		}
		totalValue := balance * price
		var trades []Trade
		trades, err = GetTradesList(symbol, limit)
		if err != nil {
			logrus.Error("Error fetching trades:", err)
			return c.JSON(400, map[string]interface{}{"Err": ErrorGenericResp.Error(), "Data": nil})
		}
		var totalCost, totalGain, totalBuyQty, totalSaleQty float64
		var totalLpTakerQty, totalLpMakerQty int

		lastBuyPrice := 0.0
		lastBuyPriceTs := 0
		highestBuyPrice := 0.0
		lowestBuyPrice := 0.0
		lastSalePrice := 0.0
		lastSalePriceTs := 0
		highestSalePrice := 0.0
		lowestSalePrice := 0.0
		var buyTrades, sellTrades []Trade
		for _, trade := range trades {
			if !trade.IsBuyer {
				sellTrades = append(sellTrades, trade)
				totalSaleQty++
			}
			if trade.IsBuyer {
				buyTrades = append(buyTrades, trade)
				totalBuyQty++
			}
			if trade.IsMaker {
				totalLpMakerQty++
			}
			if !trade.IsMaker {
				totalLpTakerQty++
			}
		}

		for i, trade := range buyTrades {
			price, _ := strconv.ParseFloat(trade.Price, 64)
			if price > highestBuyPrice {
				highestBuyPrice = price
			}
			if i == 0 || price < lowestBuyPrice {
				lowestBuyPrice = price
			}
			if i == len(buyTrades)-1 {
				lastBuyPrice = price
				lastBuyPriceTs = trade.Time
			}
			totalCost += price
		}
		for i, trade := range sellTrades {
			price, _ := strconv.ParseFloat(trade.Price, 64)
			if price > highestSalePrice {
				highestSalePrice = price
			}
			if i == 0 || price < lowestSalePrice {
				lowestSalePrice = price
			}
			if i == len(sellTrades)-1 {
				lastSalePrice = price
				lastSalePriceTs = trade.Time
			}
			totalGain += price
		}

		avgBuyPrice := totalCost / totalBuyQty
		priceChange, lastPrice, err := Get24HoursTickerPrice(symbol)
		if err != nil {
			logrus.Error("Error fetching 24-hour stats:", err)
			return c.JSON(400, map[string]interface{}{"Err": ErrorGenericResp.Error(), "Data": nil})

		}
		dailyPNL := balance * priceChange
		unrealizedPNL := (price - avgBuyPrice) * balance
		realizedPNL, err := calculateRealizedPNL(trades, avgBuyPrice)
		if err != nil {
			logrus.Error("Error calculating realized PNL:", err)
			return c.JSON(400, map[string]interface{}{"Err": ErrorGenericResp.Error(), "Data": nil})

		}
		totalPortfolioValue, err := getTotalPortfolioValue(currency)
		if err != nil {
			logrus.Error("Error calculating total portfolio value:", err)
			return c.JSON(400, map[string]interface{}{"Err": ErrorGenericResp.Error(), "Data": nil})

		}
		portfolioAllocation := (totalValue / totalPortfolioValue) * 100

		return c.JSON(200, RESTResp{Data: map[string]interface{}{
			"LAST_PRICE": lastPrice,
			"COUNT": map[string]float64{
				"BUY":  totalBuyQty,
				"SALE": totalSaleQty,
			},
			"BUY_PRICE": map[string]stas[int]{
				"LAST":    lastBuyPrice,
				"LAST_TS": lastBuyPriceTs,
				"HIGHEST": highestBuyPrice,
				"LOWEST":  lowestBuyPrice,
			},
			"SALE_PRICE": map[string]stas[int]{
				"LAST":    lastSalePrice,
				"LAST_TS": lastSalePriceTs,
				"HIGHEST": highestSalePrice,
				"LOWEST":  lowestSalePrice,
			},
			"LP_STATS": map[string]int{
				"totalLpMakerQty": totalLpMakerQty,
				"totalLpTakerQty": totalLpTakerQty,
			},
			"TOTAL_VALUE":                  totalValue,
			"AVG_BUY_PRICE":                avgBuyPrice,
			"DAILY_PNL":                    dailyPNL,
			"UNREALIZED_PNL":               unrealizedPNL,
			"REALIZED_PNL":                 realizedPNL,
			"PORTFOLIO_ALLOCATION_PERCENT": portfolioAllocation,
		}})
	})
	e.GET("/account", func(c echo.Context) error {
		data, _ := GetAccountBalances()
		return c.JSON(200, data)
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
