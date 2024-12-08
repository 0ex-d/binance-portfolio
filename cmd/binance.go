package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
)

var binanceBaseURL = "https://api.binance.com/api/v3"

type Order struct {
	Symbol                  string `json:"symbol"`
	OrderId                 int64  `json:"orderId"`
	OrderListId             int    `json:"orderListId"`
	ClientOrderId           string `json:"clientOrderId"`
	Price                   string `json:"price"`
	OrigQty                 string `json:"origQty"`
	ExecutedQty             string `json:"executedQty"`
	CummulativeQuoteQty     string `json:"cummulativeQuoteQty"`
	Status                  string `json:"status"`
	TimeInForce             string `json:"timeInForce"`
	Type                    string `json:"type"`
	Side                    string `json:"side"`
	StopPrice               string `json:"stopPrice"`
	IcebergQty              string `json:"icebergQty"`
	Time                    int    `json:"time"`
	UpdateTime              int    `json:"updateTime"`
	IsWorking               bool   `json:"isWorking"`
	WorkingTime             int    `json:"workingTime"`
	OrigQuoteOrderQty       string `json:"origQuoteOrderQty"`
	SelfTradePreventionMode string `json:"selfTradePreventionMode"`
}

type Trade struct {
	Symbol          string `json:"symbol"`
	ID              int64  `json:"id"`
	OrderId         int64  `json:"orderId"`
	OrderListId     int    `json:"orderListId"`
	Price           string `json:"price"`
	Qty             string `json:"qty"`
	QuoteQty        string `json:"quoteQty"`
	Commission      string `json:"commission"`
	CommissionAsset string `json:"commissionAsset"`
	Time            int    `json:"time"`
	IsBuyer         bool   `json:"isBuyer"`
	IsMaker         bool   `json:"isMaker"`
	IsBestMatch     bool   `json:"isBestMatch"`
}

type AccountInfo struct {
	MakerCommission            int             `json:"makerCommission"`
	TakerCommission            int             `json:"takerCommission"`
	BuyerCommission            int             `json:"buyerCommission"`
	SellerCommission           int             `json:"sellerCommission"`
	CommissionRates            CommissionRates `json:"commissionRates"`
	CanTrade                   bool            `json:"canTrade"`
	CanWithdraw                bool            `json:"canWithdraw"`
	CanDeposit                 bool            `json:"canDeposit"`
	Brokered                   bool            `json:"brokered"`
	RequireSelfTradePrevention bool            `json:"requireSelfTradePrevention"`
	PreventSor                 bool            `json:"preventSor"`
	UpdateTime                 int             `json:"updateTime"`
	AccountType                string          `json:"accountType"`
	Balances                   []Balance       `json:"balances"`
	Permissions                []string        `json:"permissions"`
	UID                        int             `json:"uid"`
}

type CommissionRates struct {
	Maker  string `json:"maker"`
	Taker  string `json:"taker"`
	Buyer  string `json:"buyer"`
	Seller string `json:"seller"`
}

type Balance struct {
	Asset  string `json:"asset"`
	Free   string `json:"free"`
	Locked string `json:"locked"`
}

func GetAllOrders(symbol string, limit string) ([]Order, error) {
	var err error
	var orders []Order
	endpoint := "/allOrders"
	apiKey, secretKey := getApiAndSecretKeys()
	timestamp := getTs()
	queryString := fmt.Sprintf("symbol=%s&limit=%s&timestamp=%s", symbol, limit, timestamp)
	signature := signParams(queryString, secretKey)
	url := fmt.Sprintf("%s%s?%s&signature=%s", binanceBaseURL, endpoint, queryString, signature)
	log.Info("[getAllOrders]: ", url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return orders, err
	}
	req.Header.Add("X-MBX-APIKEY", apiKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return orders, err
	}
	defer resp.Body.Close()
	var body []byte
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return orders, err
	}
	if resp.StatusCode != 200 {
		return orders, errors.New("")
	}
	if err := json.Unmarshal(body, &orders); err != nil {
		log.Error("error decoding JSON", err)
		return orders, err
	}
	var filteredOrders []Order
	for _, order := range orders {
		if order.Status != "FILLED" {
			continue
		}
		filteredOrders = append(filteredOrders, order)
	}
	return filteredOrders, nil
}

func Get24HoursTickerPrice(symbol string) (float64, float64, error) {
	url := fmt.Sprintf("%s/ticker/24hr?symbol=%s", binanceBaseURL, symbol)
	log.Info("[get24HoursTickerPrice]: ", url)
	resp, err := http.Get(url)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	var stats struct {
		PriceChange string `json:"priceChange"`
		LastPrice   string `json:"lastPrice"`
	}
	err = json.NewDecoder(resp.Body).Decode(&stats)
	if err != nil {
		return 0, 0, err
	}

	priceChange, err := strconv.ParseFloat(stats.PriceChange, 64)
	if err != nil {
		return 0, 0, err
	}
	lastPrice, err := strconv.ParseFloat(stats.LastPrice, 64)
	if err != nil {
		return 0, 0, err
	}
	return priceChange, lastPrice, nil
}

func GetAccount() (AccountInfo, error) {
	var err error
	var accountInfo AccountInfo
	endpoint := "/account"
	apiKey, secretKey := getApiAndSecretKeys()
	timestamp := getTs()
	queryString := fmt.Sprintf("omitZeroBalances=true&timestamp=%s", timestamp)
	signature := signParams(queryString, secretKey)
	url := fmt.Sprintf("%s%s?%s&signature=%s", binanceBaseURL, endpoint, queryString, signature)
	log.Info("[getAccount]: ", url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return accountInfo, err
	}
	req.Header.Add("X-MBX-APIKEY", apiKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return accountInfo, err
	}
	defer resp.Body.Close()
	var body []byte
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return accountInfo, err
	}
	if resp.StatusCode != 200 {
		return accountInfo, errors.New("")
	}
	if err := json.Unmarshal(body, &accountInfo); err != nil {
		log.Error("error decoding JSON", err)
		return accountInfo, err
	}
	return accountInfo, nil
}

func GetAccountBalance(asset string) (float64, error) {
	timestamp := time.Now().UnixMilli()
	queryString := fmt.Sprintf("omitZeroBalances=true&timestamp=%d", timestamp)
	apiKey, secretKey := getApiAndSecretKeys()
	signature := signParams(queryString, secretKey)

	url := fmt.Sprintf("%s/account?%s&signature=%s", binanceBaseURL, queryString, signature)
	log.Info("[getAccountBalance]: ", url)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-MBX-APIKEY", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Balances []struct {
			Asset  string `json:"asset"`
			Free   string `json:"free"`
			Locked string `json:"locked"`
		} `json:"balances"`
	}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return 0, err
	}

	for _, balance := range result.Balances {
		if balance.Asset == asset {
			freeBalance, err := strconv.ParseFloat(balance.Free, 64)
			if err != nil {
				return 0, err
			}
			return freeBalance, nil
		}
	}
	return 0, fmt.Errorf("asset %s not found in account", asset)
}

func GetCurrentTickerPrice(symbol string) (float64, error) {
	url := fmt.Sprintf("%s/ticker/price?symbol=%s", binanceBaseURL, symbol)
	log.Info("[getCurrentTickerPrice]: ", url)
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Price string `json:"price"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return 0, err
	}

	price, err := strconv.ParseFloat(result.Price, 64)
	if err != nil {
		return 0, err
	}
	return price, nil
}

func GetTradesList(symbol string, limit string) ([]Trade, error) {
	var err error
	var trades []Trade
	endpoint := "/myTrades"
	apiKey, secretKey := getApiAndSecretKeys()
	timestamp := getTs()
	queryString := fmt.Sprintf("symbol=%s&limit=%s&timestamp=%s", symbol, limit, timestamp)
	signature := signParams(queryString, secretKey)
	url := fmt.Sprintf("%s%s?%s&signature=%s", binanceBaseURL, endpoint, queryString, signature)
	log.Info("[getTradesList]: ", url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return trades, err
	}
	req.Header.Add("X-MBX-APIKEY", apiKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return trades, err
	}
	defer resp.Body.Close()
	var body []byte
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return trades, err
	}
	if resp.StatusCode != 200 {
		return trades, errors.New("")
	}
	if err := json.Unmarshal(body, &trades); err != nil {
		log.Error("error decoding JSON", err)
		return trades, err
	}
	// var filteredTrades []Trade
	// for _, trade := range trades {
	// 	// if order.Status != "FILLED" {
	// 	// 	continue
	// 	// }
	// 	filteredTrades = append(filteredTrades, trade)
	// }
	return trades, nil
}
