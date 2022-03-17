package analyticsquery

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/incognitochain/coin-service/analyticdb"
)

type PairRateV2 struct {
	Time         time.Time
	Open         float64
	Close        float64
	Average      float64
	High         float64
	Low          float64
	Trades       int
	VolumeToken1 uint64
	VolumeToken2 uint64
}

func APIGetPDexV3PairRateHistoriesV2(poolid, period, from, to string) (*PDexPairRateHistoriesAPIResponse, error) {
	if period != Period15m && period != Period1h && period != Period1d {
		return nil, errors.New("unsupported period")
	}
	queryStr := genPriceQuery(poolid, period, from, to)
	rows, err := analyticdb.ExecQuery(context.Background(), queryStr)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return nil, err
		}
	}
	var ratev2 []PairRateV2
	for rows.Next() {
		var r PairRateV2
		err = rows.Scan(&r.Time, &r.Open, &r.Close, &r.Average, &r.High, &r.Low, &r.Trades, &r.VolumeToken1, &r.VolumeToken2)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to scan %v\n", err)
			os.Exit(1)
		}
		ratev2 = append(ratev2, r)
	}
	var responseBodyData PDexPairRateHistoriesAPIResponse

	result, err := tranformsDataPairRateV2toV1(ratev2)
	if err != nil {
		return nil, err
	}
	responseBodyData.Result = result
	return &responseBodyData, nil
}

func tranformsDataPairRateV2toV1(ratev2 []PairRateV2) ([]PDexPairRate, error) {
	var result []PDexPairRate
	for _, v := range ratev2 {
		data := PDexPairRate{
			Timestamp: v.Time.Format(time.RFC3339),
			Low:       v.Low,
			High:      v.High,
			Open:      v.Open,
			Close:     v.Close,
			Average:   v.Average,
		}
		result = append(result, data)
	}
	return result, nil
}

func genPriceQuery(poolid, period, from, to string) string {
	result := `select * from `
	switch period {
	case Period15m:
		result += period15m_table
	case Period1h:
		result += period1h_table
	case Period1d:
		result += period1d_table
	}
	result += fmt.Sprintf(` where period >= '%v' and period < '%v'and sell_pool_id = '%v'`, from, to, poolid)
	return result
}

func genTradingVolume24hQuery(poolid string) string {
	return fmt.Sprintf("", poolid)
}
