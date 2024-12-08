package main

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"strings"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	log "github.com/sirupsen/logrus"
	"goland.local/binance-portfolio/pkg"
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

var walletBalancesInMemory []*pkg.WalletBalance
var portfolioBalancesInMemory []*pkg.PortfolioBalance
var assetToTradesInMemory = make(map[string][]pkg.Trade)

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
		var data []pkg.Order
		data, err = pkg.GetAllOrders(symbol, limit)
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
		var data []pkg.Trade
		data, err = pkg.GetTradesList(symbol, limit)
		return c.JSON(200, data)
	})

	e.GET("/portfolio", func(c echo.Context) error {
		var balances []*pkg.PortfolioBalance
		currency := "USDT"
		balances, err := pkg.GetPortfolioBalancesAndCCData(currency, portfolioBalancesInMemory, assetToTradesInMemory)
		if err != nil {
			return c.JSON(400, pkg.RESTResp[[]*pkg.PortfolioBalance]{Data: balances, Err: errors.New("error getting balances")})
		}
		return c.JSON(200, pkg.RESTResp[[]*pkg.PortfolioBalance]{Data: balances})
	})
	e.GET("/wallet", func(c echo.Context) error {
		var balances []*pkg.WalletBalance
		currency := "USDT"
		balances, err := pkg.GetWalletBalancesAndCCData(currency, walletBalancesInMemory)
		if err != nil {
			return c.JSON(400, pkg.RESTResp[[]*pkg.WalletBalance]{Data: balances, Err: errors.New("error getting balances")})
		}
		return c.JSON(200, pkg.RESTResp[[]*pkg.WalletBalance]{Data: balances})
	})

	e.GET("/", func(c echo.Context) error {
		return c.Render(200, "index", IndexPage{})
	})
	port := "42000"
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", port)))
}
