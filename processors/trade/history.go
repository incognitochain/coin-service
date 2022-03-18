package trade

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/incognito-chain/metadata"
)

var historyState State

func startProcessHistory() {
	err := database.DBCreateTradeIndex()
	if err != nil {
		panic(err)
	}
	err = loadState(&historyState, "trade")
	if err != nil {
		panic(err)
	}
	for {
		time.Sleep(5 * time.Second)
		startTime := time.Now()

		metas := []string{strconv.Itoa(metadata.PDECrossPoolTradeRequestMeta), strconv.Itoa(metadata.PDETradeRequestMeta), strconv.Itoa(metadata.Pdexv3TradeRequestMeta), strconv.Itoa(metadata.Pdexv3AddOrderRequestMeta), strconv.Itoa(metadata.PDECrossPoolTradeResponseMeta), strconv.Itoa(metadata.PDETradeResponseMeta), strconv.Itoa(metadata.Pdexv3TradeResponseMeta), strconv.Itoa(metadata.Pdexv3WithdrawOrderRequestMeta), strconv.Itoa(metadata.Pdexv3AddOrderResponseMeta), strconv.Itoa(metadata.Pdexv3WithdrawOrderResponseMeta)}

		txList, err := getTxToProcess(metas, historyState.LastProcessedObjectID, 10000)
		if err != nil {
			log.Println("getTxToProcess", err)
			continue
		}
		request, respond, withdrawReq, withdrawRes, err := processTradeToken(txList)
		if err != nil {
			panic(err)
		}

		fmt.Println("len(request)", len(request), len(respond), len(withdrawReq), len(withdrawRes), len(txList), time.Since(startTime))
		err = database.DBSaveTradeOrder(request)
		if err != nil {
			panic(err)
		}
		err = database.DBUpdateTradeOrder(respond)
		if err != nil {
			panic(err)
		}
		err = database.DBUpdateWithdrawTradeOrderReq(withdrawReq)
		if err != nil {
			panic(err)
		}

		err = database.DBUpdateWithdrawTradeOrderRes(withdrawRes)
		if err != nil {
			panic(err)
		}

		fmt.Println("mongo time", time.Since(startTime))
		if len(txList) != 0 {
			historyState.LastProcessedObjectID = txList[len(txList)-1].ID.Hex()
			err = updateState(&historyState, "trade")
			if err != nil {
				panic(err)
			}
		}

		err = updateTradeStatus()
		if err != nil {
			panic(err)
		}

		fmt.Println("process time", time.Since(startTime))
	}
}
