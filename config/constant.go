package config

import (
	"github.com/shopspring/decimal"
	"github.com/tidwall/gjson"
	"io/ioutil"
	"net/http"
	"time"
)

const (
	Init_node_chain_hash = "1EXoDusjGwvnjZUyKkxZ4UHEf77z6A5S4P"
	BtcNeedFundTimes     = 3
	//Dust                 = 0.00000540
)

//database
const (
	DBname        = "obdserver.db"
	TrackerDbName = "trackerServer.db"
)

func GetHtlcFee() float64 {
	return 0.00001
}

var minerFeePricePerByte = 0.0
var successGetMinerFeePriceAt time.Time

func httpGetRecommendedMiner() (price float64) {

	if successGetMinerFeePriceAt.IsZero() == false {
		now := time.Now().Add(-6 * time.Hour)
		if now.Before(successGetMinerFeePriceAt) {
			return minerFeePricePerByte
		}
	}
	url := "https://bitcoinfees.earn.com/api/v1/fees/recommended"
	client := http.Client{Timeout: time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		successGetMinerFeePriceAt = time.Now()
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		successGetMinerFeePriceAt = time.Now()
		minerFeePricePerByte = gjson.Get(string(body), "hourFee").Float()
		return minerFeePricePerByte
	}
	return 0
}

func GetMinMinerFee(ins int) float64 {
	txSize := ins*150 + 68 + 90
	result, _ := decimal.NewFromFloat(float64(txSize) * 3.5).Div(decimal.NewFromFloat(100000000)).Round(8).Float64()
	return result
}

func GetOmniDustBtc() float64 {
	return 0.0000054
}
