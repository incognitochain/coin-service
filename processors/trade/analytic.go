package trade

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/metadata"
)

var analyticState State

func startAnalytic() {
	err := loadState(&historyState, "trade-analytic")
	if err != nil {
		panic(err)
	}
	for {
		time.Sleep(5 * time.Second)
		startTime := time.Now()

		metas := []string{strconv.Itoa(metadata.PDECrossPoolTradeRequestMeta), strconv.Itoa(metadata.PDETradeRequestMeta), strconv.Itoa(metadata.Pdexv3TradeRequestMeta), strconv.Itoa(metadata.Pdexv3AddOrderRequestMeta), strconv.Itoa(metadata.PDECrossPoolTradeResponseMeta), strconv.Itoa(metadata.PDETradeResponseMeta), strconv.Itoa(metadata.Pdexv3TradeResponseMeta), strconv.Itoa(metadata.Pdexv3WithdrawOrderRequestMeta), strconv.Itoa(metadata.Pdexv3WithdrawOrderResponseMeta)}

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

func extractDataForAnalytic(txlist []shared.TxData) {}
