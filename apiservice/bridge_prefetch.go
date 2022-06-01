package apiservice

import (
	"log"
	"time"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/incognito-chain/blockchain/bridgeagg"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
)

var bridgeState *bridgeagg.State

func bridgeStateWatcher() {
	for {
		retrieveBridgeState()
		time.Sleep(15 * time.Second)
	}
}

func retrieveBridgeState() {
	state, err := database.DBGetBridgeState(1)
	if err != nil {
		log.Println(err)
		return
	}
	bridgeStateJson := jsonresult.BridgeAggState{}
	err = json.UnmarshalFromString(state, &bridgeStateJson)
	if err != nil {
		log.Println(err)
		return
	}
	bridgeState = bridgeagg.NewStateWithValue(bridgeStateJson.UnifiedTokenInfos, bridgeStateJson.WaitingUnshieldReqs, bridgeStateJson.Param, nil, nil)
}
