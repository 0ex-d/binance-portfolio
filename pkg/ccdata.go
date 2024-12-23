package pkg

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

var ccDataBaseURL = "https://data-api.ccdata.io"

type CCDataResponse struct {
	Data map[string]CCDataSpotInstrumentData `json:"Data"`
}

type CCDataSpotInstrumentData struct {
	Type                       string  `json:"TYPE"`
	Market                     string  `json:"MARKET"`
	Instrument                 string  `json:"INSTRUMENT"`
	CcSeq                      int     `json:"CCSEQ"`
	Price                      float64 `json:"PRICE"`
	PriceFlag                  string  `json:"PRICE_FLAG"`
	PriceLastUpdateTs          int64   `json:"PRICE_LAST_UPDATE_TS"`
	PriceLastUpdateTsNs        int64   `json:"PRICE_LAST_UPDATE_TS_NS"`
	CurrentDayChange           float64 `json:"CURRENT_DAY_CHANGE"`
	CurrentDayChangePercentage float64 `json:"CURRENT_DAY_CHANGE_PERCENTAGE"`
}

func GetCCDataCurrentTickerPrice(instruments, apiKey string) (CCDataResponse, error) {
	startTs := time.Now()
	url := fmt.Sprintf("%s/spot/v1/latest/tick?market=binance&instruments=%s&apply_mapping=false&groups=ID,VALUE,CURRENT_DAY&api_key=%s", ccDataBaseURL, instruments, apiKey)
	log.Info("[GetCCDataCurrentTickerPrice]: ", url)
	data := CCDataResponse{}
	resp, err := http.Get(url)
	if err != nil {
		return data, err
	}
	defer resp.Body.Close()
	log.Infof("[GetCCDataCurrentTickerPrice]: took: %v seconds", time.Since(startTs).Seconds())
	if resp.StatusCode != 200 {
		return data, errors.New("error fetching from ccdata.io")
	}
	var result CCDataResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return data, err
	}
	return result, nil
}
