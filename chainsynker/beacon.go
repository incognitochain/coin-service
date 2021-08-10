package chainsynker

import (
	"fmt"
	"log"
	"time"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/blockchain/pdex"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/dataaccessobject/rawdbv2"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
)

func processBeacon(bc *blockchain.BlockChain, h common.Hash, height uint64) {
	bestHeight := Localnode.GetBlockchain().GetBeaconBestState().BeaconHeight
	doProcess := false
	if bestHeight-height < 8 {
		doProcess = true
	}

	beaconBestState, _ := Localnode.GetBlockchain().GetBeaconViewStateDataFromBlockHash(h, false)
	blk := beaconBestState.BestBlock
	beaconFeatureStateRootHash := beaconBestState.FeatureStateDBRootHash
	beaconFeatureStateDB, err := statedb.NewWithPrefixTrie(beaconFeatureStateRootHash, statedb.NewDatabaseAccessWarper(Localnode.GetBlockchain().GetBeaconChainDatabase()))
	if err != nil {
		log.Println(err)
	}

	log.Printf("start processing coin for block %v beacon\n", blk.GetHeight())
	startTime := time.Now()

	// Process PDEstate
	pdeState, err := pdex.InitStateFromDB(beaconFeatureStateDB, beaconBestState.BeaconHeight)
	if err != nil {
		log.Println(err)
	}

	var stateV1 *shared.PDEStateV1
	var stateV2 *shared.PDEStateV2

	if pdeState.Version() == 1 {
		poolPairs := make(map[string]*rawdbv2.PDEPoolForPair)
		waitingContributions := make(map[string]*rawdbv2.PDEContribution)
		err = json.Unmarshal(pdeState.Reader().WaitingContributions(), &waitingContributions)
		if err != nil {
			panic(err)
		}
		err = json.Unmarshal(pdeState.Reader().PoolPairs(), &poolPairs)
		if err != nil {
			panic(err)
		}

		stateV1 = &shared.PDEStateV1{
			WaitingContributions: waitingContributions,
			PDEPoolPairs:         poolPairs,
			PDEShares:            pdeState.Reader().Shares(),
			PDETradingFees:       pdeState.Reader().TradingFees(),
		}
	} else {
		poolPairs := make(map[string]*pdex.PoolPairState)
		waitingContributions := make(map[string]*rawdbv2.Pdexv3Contribution)
		err = json.Unmarshal(pdeState.Reader().WaitingContributions(), &waitingContributions)
		if err != nil {
			panic(err)
		}
		err = json.Unmarshal(pdeState.Reader().PoolPairs(), &poolPairs)
		if err != nil {
			panic(err)
		}

		stateV2 = &shared.PDEStateV2{
			WaitingContributions: waitingContributions,
			PoolPairs:            poolPairs,
			// StakingPoolsState:    pdeState.Reader().
		}
	}
	// newPDEState := shared.CurrentPDEState{
	// 	Version:         pdeState.Version(),
	// 	StateV1:         *stateV1,
	// 	StateV2:         *stateV2,
	// 	BeaconTimeStamp: beaconBestState.GetBlockTime(),
	// }
	// pdeStr, err := json.MarshalToString(newPDEState)
	// if err != nil {
	// 	log.Println(err)
	// }
	// err = database.DBSavePDEState(pdeStr)
	// if err != nil {
	// 	log.Println(err)
	// }
	orderStatusList := []shared.TradeOrderData{}
	waitingContributeList := []shared.WaitingContributions{}
	if pdeState.Version() == 1 {
		if doProcess {
			waitingContributeList = processWaitingContribute(stateV1.WaitingContributions, nil, height)
		}

	} else {
		if doProcess {
			waitingContributeList = processWaitingContribute(nil, stateV2.WaitingContributions, height)
		}
		orderStatusList = processBeaconOrder(stateV2.Orders)
		pairData := []shared.PairData{}
		pools := []shared.PoolPairData{}

		for poolID, poolPair := range stateV2.PoolPairs {
			_ = poolID
			_ = poolPair
		}

		_ = pairData
		err = database.DBSavePoolPairs(pools)
		if err != nil {
			log.Println(err)
		}
	}
	_ = orderStatusList
	_ = waitingContributeList
	statePrefix := BeaconData
	err = Localnode.GetUserDatabase().Put([]byte(statePrefix), []byte(fmt.Sprintf("%v", blk.Header.Height)), nil)
	if err != nil {
		panic(err)
	}
	log.Printf("finish processing coin for block %v beacon in %v\n", blk.GetHeight(), time.Since(startTime))
}

func processWaitingContribute(statev1 map[string]*rawdbv2.PDEContribution, statev2 map[string]*rawdbv2.Pdexv3Contribution, beaconHeight uint64) []shared.WaitingContributions {
	result := []shared.WaitingContributions{}
	if statev1 != nil {
		for _, contribute := range statev1 {
			newContr := shared.WaitingContributions{
				RequestTx:     contribute.TxReqID.String(),
				RefundAddress: contribute.ContributorAddressStr,
				TokenID:       contribute.TokenIDStr,
				BeaconHeight:  beaconHeight,
			}
			result = append(result, newContr)
		}
	}
	if statev2 != nil {
		for _, contribute := range statev2 {
			newContr := shared.WaitingContributions{
				RequestTx:      contribute.TxReqID().String(),
				RefundAddress:  contribute.RefundAddress(),
				ReceiveAddress: contribute.ReceiveAddress(),
				TokenID:        contribute.TokenID().String(),
				Amount:         contribute.Amount(),
				AMP:            contribute.Amplifier(),
				BeaconHeight:   beaconHeight,
			}
			result = append(result, newContr)
		}
	}

	return result
}

func processBeaconOrder(orderGroups map[int64][]rawdbv2.Pdexv3Order) []shared.TradeOrderData {
	var result []shared.TradeOrderData
	for _, orders := range orderGroups {
		for _, order := range orders {
			newOrder := shared.TradeOrderData{
				RequestTx: order.Id(),
			}
			result = append(result, newOrder)
		}
	}

	return result
}

func processPoolPairs() {

}
