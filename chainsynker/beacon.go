package chainsynker

import (
	"fmt"
	"log"
	"sort"
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

	beaconBestState, _ := Localnode.GetBlockchain().GetBeaconViewStateDataFromBlockHash(h, false)
	blk := beaconBestState.BestBlock
	beaconFeatureStateRootHash := beaconBestState.FeatureStateDBRootHash
	beaconFeatureStateDB, err := statedb.NewWithPrefixTrie(beaconFeatureStateRootHash, statedb.NewDatabaseAccessWarper(Localnode.GetBlockchain().GetBeaconChainDatabase()))
	if err != nil {
		log.Println(err)
	}
	// this is a requirement check
	for shardID, blks := range blk.Body.ShardState {
		sort.Slice(blks, func(i, j int) bool { return blks[i].Height > blks[j].Height })
	retry:
		pass := true
		blockProcessedLock.RLock()
		if blockProcessed[int(shardID)] < blks[0].Height {
			pass = false
		}
		blockProcessedLock.RUnlock()
		if !pass {
			time.Sleep(1 * time.Second)
			goto retry
		}
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
		err = json.Unmarshal(pdeState.Reader().PoolPairs(), &poolPairs)
		if err != nil {
			panic(err)
		}

		stateV1 = &shared.PDEStateV1{
			PDEPoolPairs:   poolPairs,
			PDEShares:      pdeState.Reader().Shares(),
			PDETradingFees: pdeState.Reader().TradingFees(),
		}
	} else {
		poolPairs := make(map[string]*shared.PoolPairState)
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
			PoolPairs: poolPairs,
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
	orderStatusList := []shared.LimitOrderStatus{}

	poolDatas, sharesDatas, err := processPoolPairs(stateV1, stateV2, beaconBestState.BeaconHeight)
	if err != nil {
		panic(err)
	}

	err = database.DBUpdatePDEPoolPairData(poolDatas)
	if err != nil {
		panic(err)
	}

	err = database.DBUpdatePDEPoolShareData(sharesDatas)
	if err != nil {
		panic(err)
	}

	if pdeState.Version() == 2 {
		orderStatusList = processBeaconOrder(stateV2.Orders)
		err = database.DBUpdateOrderProgress(orderStatusList)
		if err != nil {
			panic(err)
		}
	}
	_ = orderStatusList
	statePrefix := BeaconData
	err = Localnode.GetUserDatabase().Put([]byte(statePrefix), []byte(fmt.Sprintf("%v", blk.Header.Height)), nil)
	if err != nil {
		panic(err)
	}
	blockProcessedLock.Lock()
	blockProcessed[-1] = blk.Header.Height
	blockProcessedLock.Unlock()
	log.Printf("finish processing coin for block %v beacon in %v\n", blk.GetHeight(), time.Since(startTime))
}

func processBeaconOrder(orderGroups map[int64][]rawdbv2.Pdexv3Order) []shared.LimitOrderStatus {
	var result []shared.LimitOrderStatus
	for _, orders := range orderGroups {
		for _, order := range orders {
			newOrder := shared.LimitOrderStatus{
				RequestTx: order.Id(),
				Status:    "",
				Left:      order.Token0Balance(),
			}
			result = append(result, newOrder)
		}
	}

	return result
}

func processPoolPairs(statev1 *shared.PDEStateV1, statev2 *shared.PDEStateV2, beaconHeight uint64) ([]shared.PoolPairData, []shared.PoolShareData, error) {
	var poolPairs []shared.PoolPairData
	var poolShare []shared.PoolShareData

	for poolID, state := range statev1.PDEPoolPairs {
		poolData := shared.PoolPairData{
			Version:      1,
			PoolID:       poolID,
			PairID:       state.Token1IDStr + "-" + state.Token2IDStr,
			TokenID1:     state.Token1IDStr,
			TokenID2:     state.Token2IDStr,
			Token1Amount: state.Token1PoolValue,
			Token2Amount: state.Token2PoolValue,
		}
		poolPairs = append(poolPairs, poolData)
	}

	for shareID, share := range statev1.PDEShares {
		shareData := shared.PoolShareData{
			Version:    1,
			Amount:     share,
			TradingFee: map[string]uint64{common.PRVCoinID.String(): statev1.PDETradingFees[shareID]},
			Pubkey:     "",
		}
		poolShare = append(poolShare, shareData)
	}

	for poolID, state := range statev2.PoolPairs {
		for shareID, share := range state.Shares {
			shareData := shared.PoolShareData{
				Version:    2,
				Amount:     share.Amount(),
				TradingFee: share.TradingFees(),
				NFTID:      shareID,
			}
			poolShare = append(poolShare, shareData)
		}

		poolData := shared.PoolPairData{
			Version:      2,
			PoolID:       poolID,
			PairID:       state.State.Token0ID().String() + "-" + state.State.Token1ID().String(),
			AMP:          state.State.Amplifier(),
			TokenID1:     state.State.Token0ID().String(),
			TokenID2:     state.State.Token1ID().String(),
			Token1Amount: state.State.Token0RealAmount(),
			Token2Amount: state.State.Token1RealAmount(),
		}
		poolPairs = append(poolPairs, poolData)
	}

	// for nftID, share := range statev2.StakingPoolsState {

	// }

	return poolPairs, poolShare, nil
}
