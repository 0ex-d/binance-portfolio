package pkg

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

type Prices []float64

type stas[T float64 | int] interface{}
type RESTResp[T interface{} | map[string]interface{}] struct {
	Data T
	Err  interface{}
}

type WalletBalance struct {
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

type PortfolioBalance struct {
	Symbol             string              `json:"symbol"`
	QuoteSymbol        string              `json:"quote_symbol"`
	Free               float64             `json:"free"`
	Locked             float64             `json:"locked"`
	Price              float64             `json:"price"`
	PriceFlag          string              `json:"price_flag"`
	PriceChangeValue   float64             `json:"price_change_value"`
	PriceChangePercent float64             `json:"price_change_percent"`
	QuoteValue         float64             `json:"quote_value"`
	TradeStats         PortfolioTradeStats `json:"trade_stats"`
}

type TradePriceAndTimestamp struct {
	Price     float64
	Timestamp int
}
type PortfolioTradeSideStats struct {
	Last    TradePriceAndTimestamp
	Highest TradePriceAndTimestamp
	Lowest  TradePriceAndTimestamp
	Qty     float64
}
type PortfolioTradeStats struct {
	Buy  PortfolioTradeSideStats
	Sale PortfolioTradeSideStats
}

func signParams(message, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

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

func calculateTradeCosts(trades []Trade) PortfolioTradeStats {
	var totalCost, totalGain, totalBuyQty, totalSaleQty float64
	var totalLpTakerQty, totalLpMakerQty int

	lastBuyPrice := 0.0
	lastBuyPriceTs := 0
	highestBuyPrice := 0.0
	highestBuyPriceTs := 0
	lowestBuyPrice := 0.0
	lowestBuyPriceTs := 0
	lastSalePrice := 0.0
	lastSalePriceTs := 0
	highestSalePrice := 0.0
	highestSalePriceTs := 0
	lowestSalePrice := 0.0
	lowestSalePriceTs := 0
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
			highestBuyPriceTs = trade.Time
		}
		if i == 0 || price < lowestBuyPrice {
			lowestBuyPrice = price
			lowestBuyPriceTs = trade.Time
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
			highestSalePriceTs = trade.Time
		}
		if i == 0 || price < lowestSalePrice {
			lowestSalePrice = price
			lowestSalePriceTs = trade.Time
		}
		if i == len(sellTrades)-1 {
			lastSalePrice = price
			lastSalePriceTs = trade.Time
		}
		totalGain += price
	}
	portfolioTradeStats := PortfolioTradeStats{
		Buy: PortfolioTradeSideStats{
			Last: TradePriceAndTimestamp{
				Price:     lastBuyPrice,
				Timestamp: lastBuyPriceTs,
			},
			Highest: TradePriceAndTimestamp{
				Price:     highestBuyPrice,
				Timestamp: highestBuyPriceTs,
			},
			Lowest: TradePriceAndTimestamp{
				Price:     lowestBuyPrice,
				Timestamp: lowestBuyPriceTs,
			},
			Qty: totalBuyQty,
		},
		Sale: PortfolioTradeSideStats{
			Last: TradePriceAndTimestamp{
				Price:     lastSalePrice,
				Timestamp: lastSalePriceTs,
			},
			Highest: TradePriceAndTimestamp{
				Price:     highestSalePrice,
				Timestamp: highestSalePriceTs,
			},
			Lowest: TradePriceAndTimestamp{
				Price:     lowestSalePrice,
				Timestamp: lowestSalePriceTs,
			},
			Qty: totalBuyQty,
		},
	}

	return portfolioTradeStats
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

func GetWalletBalancesAndCCData(currency string, walletBalancesInMemory []*WalletBalance) ([]*WalletBalance, error) {
	var err error
	var balances []Balance
	var portfolioBalances []*WalletBalance
	if len(walletBalancesInMemory) != 0 {
		log.Info("[getWalletBalancesAndCCData]: Getting from memory")
		return walletBalancesInMemory, err
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
			portfolioBalances = append(portfolioBalances, &WalletBalance{
				Symbol: balance.Asset,
				Free:   balance.Free,
			})
			continue
		}
		instrument := fmt.Sprintf("%s-%s", balance.Asset, currency)
		currentInstrument := spotResponse.Data[instrument]
		assetValue := balance.Free * currentInstrument.Price
		portfolioBalances = append(portfolioBalances, &WalletBalance{
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
	walletBalancesInMemory = portfolioBalances
	return portfolioBalances, nil
}

func GetPortfolioBalancesAndCCData(currency string, portfolioBalancesInMemory []*PortfolioBalance, assetToTradesInMemory map[string][]Trade) ([]*PortfolioBalance, error) {
	var err error
	var balances []Balance
	var portfolioBalances []*PortfolioBalance
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
			portfolioBalances = append(portfolioBalances, &PortfolioBalance{
				Symbol: balance.Asset,
				Free:   balance.Free,
			})
			continue
		}
		ccInstrument := fmt.Sprintf("%s-%s", balance.Asset, currency)
		binanceInstrument := fmt.Sprintf("%s%s", balance.Asset, currency)
		currentInstrument := spotResponse.Data[ccInstrument]
		assetValue := balance.Free * currentInstrument.Price

		if _, ok := assetToTradesInMemory[binanceInstrument]; !ok {
			log.Warnf("%s: not in memory. fetching API", binanceInstrument)
			assetTrades, err := GetTradesList(binanceInstrument, "1000")
			if err != nil {
				log.Errorf("%s: Error fetching trades: %v", binanceInstrument, err)
				continue
			}
			assetToTradesInMemory[binanceInstrument] = assetTrades
		} else {
			log.Infof("%s: fetching from memory.", binanceInstrument)
		}
		assetTradesMemStore := assetToTradesInMemory[binanceInstrument]
		tradeStats := calculateTradeCosts(assetTradesMemStore)
		// var totalCost, totalGain, totalBuyQty, totalSaleQty float64
		// var totalLpTakerQty, totalLpMakerQty int

		portfolioBalances = append(portfolioBalances, &PortfolioBalance{
			Symbol:             balance.Asset,
			QuoteSymbol:        currency,
			Free:               balance.Free,
			QuoteValue:         assetValue,
			Price:              currentInstrument.Price,
			PriceFlag:          currentInstrument.PriceFlag,
			PriceChangeValue:   currentInstrument.CurrentDayChange,
			PriceChangePercent: currentInstrument.CurrentDayChangePercentage,
			TradeStats:         tradeStats,
		})
	}
	sort.Slice(portfolioBalances, func(i, j int) bool {
		return portfolioBalances[i].Free > portfolioBalances[j].Free
	})
	portfolioBalancesInMemory = portfolioBalances
	return portfolioBalances, nil
}
