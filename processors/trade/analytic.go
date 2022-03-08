package trade

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/incognitochain/coin-service/analyticdb"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/metadata"
)

var analyticState State

func startAnalytic() {
	err := loadState(&historyState, "trade-analytic")
	if err != nil {
		panic(err)
	}
	//create trade-result-table
	for {
		time.Sleep(5 * time.Second)
		startTime := time.Now()

		metas := []string{strconv.Itoa(metadata.PDECrossPoolTradeResponseMeta), strconv.Itoa(metadata.PDETradeResponseMeta), strconv.Itoa(metadata.Pdexv3TradeResponseMeta), strconv.Itoa(metadata.Pdexv3WithdrawOrderResponseMeta)}

		txList, err := getTxToProcess(metas, historyState.LastProcessedObjectID, 10000)
		if err != nil {
			log.Println("getTxToProcess", err)
			continue
		}
		extractDataForAnalytic(txList)

		fmt.Println("mongo time", time.Since(startTime))
		if len(txList) != 0 {
			historyState.LastProcessedObjectID = txList[len(txList)-1].ID.Hex()
			err = updateState(&historyState, "trade-analytic")
			if err != nil {
				panic(err)
			}
		}

		fmt.Println("process-analytic time", time.Since(startTime))
	}
}

func extractDataForAnalytic(txlist []shared.TxData) {
	for _, tx := range txlist {

	}
}

func createAnalyticTable() error {
	queryCreateHypertable := `CREATE TABLE IF NOT EXISTS trade_data (
		time TIMESTAMPTZ NOT NULL,
		trade_id INTEGER,
		rate DOUBLE PRECISION
		pool_ids 
		);
		SELECT create_hypertable('trade_data', 'time');
		`
	_ = queryCreateHypertable
	return nil
}

func insertDataToAnalyticDB() error {
	return nil
}

func createContinuousView() error {
	var err error
	table := "trade_result_table"
	//create view
	//15m view
	queryCreateView15m := genCAViewQuery(view15m, table, period15m)
	//30m view
	queryCreateView30m := genCAViewQuery(view30m, table, period30m)
	//1h view
	queryCreateView1h := genCAViewQuery(view1h, table, period1h)
	//4h view
	queryCreateView4h := genCAViewQuery(view4h, table, period4h)
	//1d view
	queryCreateView1d := genCAViewQuery(view1d, table, period1d)
	//1w view
	queryCreateView1w := genCAViewQuery(view1w, table, period1w)

	//create auto-agg interval
	//15m view
	queryContinuousAgg15m := genConAggPolicyQuery(view15m)
	//30m view
	queryContinuousAgg30m := genConAggPolicyQuery(view30m)
	//1h view
	queryContinuousAgg1h := genConAggPolicyQuery(view1h)
	//4h view
	queryContinuousAgg4h := genConAggPolicyQuery(view4h)
	//1d view
	queryContinuousAgg1d := genConAggPolicyQuery(view1d)
	//1w view
	queryContinuousAgg1w := genConAggPolicyQuery(view1w)

	_, err = analyticdb.Exec(context.Background(), queryCreateView15m)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}
	_, err = analyticdb.Exec(context.Background(), queryCreateView30m)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}
	_, err = analyticdb.Exec(context.Background(), queryCreateView1h)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}
	_, err = analyticdb.Exec(context.Background(), queryCreateView4h)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}
	_, err = analyticdb.Exec(context.Background(), queryCreateView1d)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}
	_, err = analyticdb.Exec(context.Background(), queryCreateView1w)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}

	_, err = analyticdb.Exec(context.Background(), queryContinuousAgg15m)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}
	_, err = analyticdb.Exec(context.Background(), queryContinuousAgg30m)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}
	_, err = analyticdb.Exec(context.Background(), queryContinuousAgg1h)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}
	_, err = analyticdb.Exec(context.Background(), queryContinuousAgg4h)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}
	_, err = analyticdb.Exec(context.Background(), queryContinuousAgg1d)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}
	_, err = analyticdb.Exec(context.Background(), queryContinuousAgg1w)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}
	return nil
}
