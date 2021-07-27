package chainsynker

import (
	"log"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/blockchain/pdex"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
)

func updateBeaconState(bc *blockchain.BlockChain, h common.Hash, height uint64) {
	beaconBestState, _ := Localnode.GetBlockchain().GetBeaconViewStateDataFromBlockHash(h, false)
	beaconFeatureStateRootHash := beaconBestState.FeatureStateDBRootHash
	beaconFeatureStateDB, err := statedb.NewWithPrefixTrie(beaconFeatureStateRootHash, statedb.NewDatabaseAccessWarper(Localnode.GetBlockchain().GetBeaconChainDatabase()))
	if err != nil {
		log.Println(err)
	}
	// PDEstate
	pdeState, err := pdex.InitStateFromDB(beaconFeatureStateDB, beaconBestState.BeaconHeight)
	if err != nil {
		log.Println(err)
	}

	var stateV1 *shared.PDEStateV1
	var stateV2 *shared.PDEStateV2

	if pdeState.Version() == 1 {
		stateV1 = &shared.PDEStateV1{
			WaitingPDEContributions: pdeState.Reader().WaitingContributionsV1(),
			PDEPoolPairs:            pdeState.Reader().PoolPairsV1(),
			PDEShares:               pdeState.Reader().Shares(),
			PDETradingFees:          pdeState.Reader().TradingFees(),
		}
	} else {
		stateV2 = &shared.PDEStateV2{
			WaitingContributions: pdeState.Reader().WaitingContributionsV2(),
			PoolPairs:            pdeState.Reader().PoolPairsV2(),
			// StakingPoolsState:    pdeState.Reader().Shares(),
		}
	}

	newPDEState := shared.CurrentPDEState{
		Version:         pdeState.Version(),
		StateV1:         *stateV1,
		StateV2:         *stateV2,
		BeaconTimeStamp: beaconBestState.GetBlockTime(),
	}
	pdeStr, err := json.MarshalToString(newPDEState)
	if err != nil {
		log.Println(err)
	}
	err = database.DBSavePDEState(pdeStr)
	if err != nil {
		log.Println(err)
	}
}
