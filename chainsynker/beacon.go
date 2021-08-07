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
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
)

func processBeacon(bc *blockchain.BlockChain, h common.Hash, height uint64) {

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
		stateV1 = &shared.PDEStateV1{
			WaitingContributions: pdeState.Reader().WaitingContributionsV1(),
			PDEPoolPairs:         pdeState.Reader().PoolPairsV1(),
			PDEShares:            pdeState.Reader().Shares(),
			PDETradingFees:       pdeState.Reader().TradingFees(),
		}
	} else {
		stateV2 = &shared.PDEStateV2{
			WaitingContributions: pdeState.Reader().WaitingContributionsV2(),
			PoolPairs:            pdeState.Reader().PoolPairsV2(),
			// StakingPoolsState:    pdeState.Reader().Shares(),
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

	if pdeState.Version() == 1 {
		for key, contribute := range stateV1.WaitingContributions {

		}
	} else {
		for key, contribute := range stateV2.WaitingContributions {

		}

		pairData := []shared.PairData{}
		pools := []shared.PoolPairData{}
		for pair, poolPairs := range stateV2.PoolPairs {

		}
		_ = pairData
		err = database.DBSavePoolPairs(pools)
		if err != nil {
			log.Println(err)
		}
for _, orders := range stateV2.Orders {
	for _, order := range orders {
		order.
	}
}
	}

	statePrefix := BeaconData
	err = Localnode.GetUserDatabase().Put([]byte(statePrefix), []byte(fmt.Sprintf("%v", blk.Header.Height)), nil)
	if err != nil {
		panic(err)
	}
	log.Printf("finish processing coin for block %v beacon in %v\n", blk.GetHeight(), time.Since(startTime))
}
