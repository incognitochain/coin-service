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
	"github.com/incognitochain/incognito-chain/config"
	"github.com/incognitochain/incognito-chain/dataaccessobject/rawdbv2"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
)

func processBeacon(bc *blockchain.BlockChain, h common.Hash, height uint64) {
	log.Printf("start processing coin for block %v beacon\n", height)
	startTime := time.Now()
	beaconBestState, _ := Localnode.GetBlockchain().GetBeaconViewStateDataFromBlockHash(h, false)
	blk := beaconBestState.BestBlock
	beaconFeatureStateRootHash := beaconBestState.FeatureStateDBRootHash
	beaconFeatureStateDB, err := statedb.NewWithPrefixTrie(beaconFeatureStateRootHash, statedb.NewDatabaseAccessWarper(Localnode.GetBlockchain().GetBeaconChainDatabase()))
	if err != nil {
		log.Println(err)
	}
	var prevStateV2 *shared.PDEStateV2
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
	if height != 1 {
		poolPairs := make(map[string]*shared.PoolPairState)
		prevBeaconFeatureStateDB, err := Localnode.GetBlockchain().GetBestStateBeaconFeatureStateDBByHeight(height-1, Localnode.GetBlockchain().GetBeaconChainDatabase())
		if err != nil {
			log.Println(err)
		}

		pdeState, err := pdex.InitStateFromDB(prevBeaconFeatureStateDB, height-1, 2)
		if err != nil {
			log.Println(err)
		}
		if pdeState.Version() == 2 {
			err = json.Unmarshal(pdeState.Reader().PoolPairs(), &poolPairs)
			if err != nil {
				panic(err)
			}
			prevStateV2 = &shared.PDEStateV2{
				PoolPairs:         poolPairs,
				StakingPoolsState: pdeState.Reader().StakingPools(),
			}
		}

	}
	// Process PDEstate
	stateV1, err := pdex.InitStateFromDB(beaconFeatureStateDB, beaconBestState.BeaconHeight, 1)
	if err != nil {
		log.Println(err)
	}

	// var stateV1 *shared.PDEStateV1
	var stateV2 *shared.PDEStateV2

	// if pdeState.Version() == 1 {
	poolPairs := make(map[string]*rawdbv2.PDEPoolForPair)
	err = json.Unmarshal(stateV1.Reader().PoolPairs(), &poolPairs)
	if err != nil {
		panic(err)
	}
	waitingContributions := make(map[string]*rawdbv2.PDEContribution)
	err = json.Unmarshal(stateV1.Reader().WaitingContributions(), &waitingContributions)
	if err != nil {
		panic(err)
	}
	pdeStateJSON := jsonresult.CurrentPDEState{
		BeaconTimeStamp:         beaconBestState.BestBlock.Header.Timestamp,
		PDEPoolPairs:            poolPairs,
		PDEShares:               stateV1.Reader().Shares(),
		WaitingPDEContributions: waitingContributions,
		PDETradingFees:          stateV1.Reader().TradingFees(),
	}
	pdeStr, err := json.MarshalToString(pdeStateJSON)
	if err != nil {
		log.Println(err)
	}
	err = database.DBSavePDEState(pdeStr)
	if err != nil {
		log.Println(err)
	}

	//process stateV2
	if beaconBestState.BeaconHeight > config.Param().PDexParams.Pdexv3BreakPointHeight {
		pdeStateV2, err := pdex.InitStateFromDB(beaconFeatureStateDB, beaconBestState.BeaconHeight, 2)
		if err != nil {
			log.Println(err)
		}
		poolPairs := make(map[string]*shared.PoolPairState)
		err = json.Unmarshal(pdeStateV2.Reader().PoolPairs(), &poolPairs)
		if err != nil {
			panic(err)
		}

		stateV2 = &shared.PDEStateV2{
			PoolPairs:         poolPairs,
			StakingPoolsState: pdeStateV2.Reader().StakingPools(),
		}

		pairDatas, poolDatas, sharesDatas, poolStakeDatas, poolStakersDatas, orderBook, poolDatasToBeDel, sharesDatasToBeDel, poolStakeDatasToBeDel, poolStakersDatasToBeDel, orderBookToBeDel, err := processPoolPairs(stateV2, prevStateV2, beaconBestState.BeaconHeight)
		if err != nil {
			panic(err)
		}

		err = database.DBUpdatePDEPairListData(pairDatas)
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

		err = database.DBUpdatePDEPoolStakeData(poolStakeDatas)
		if err != nil {
			panic(err)
		}

		err = database.DBUpdatePDEPoolStakerData(poolStakersDatas)
		if err != nil {
			panic(err)
		}

		err = database.DBUpdateOrderProgress(orderBook)
		if err != nil {
			panic(err)
		}

		err = database.DBDeletePDEPoolData(poolDatasToBeDel)
		if err != nil {
			panic(err)
		}

		err = database.DBDeletePDEPoolShareData(sharesDatasToBeDel)
		if err != nil {
			panic(err)
		}

		err = database.DBDeletePDEPoolStakeData(poolStakeDatasToBeDel)
		if err != nil {
			panic(err)
		}

		err = database.DBDeletePDEPoolStakerData(poolStakersDatasToBeDel)
		if err != nil {
			panic(err)
		}

		err = database.DBDeleteOrderProgress(orderBookToBeDel)
		if err != nil {
			panic(err)
		}

	}

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

func processPoolPairs(statev2 *shared.PDEStateV2, prevStatev2 *shared.PDEStateV2, beaconHeight uint64) ([]shared.PairData, []shared.PoolPairData, []shared.PoolShareData, []shared.PoolStakeData, []shared.PoolStakerData, []shared.LimitOrderStatus, []shared.PoolPairData, []shared.PoolShareData, []shared.PoolStakeData, []shared.PoolStakerData, []shared.LimitOrderStatus, error) {
	var pairList []shared.PairData
	pairListMap := make(map[string][]shared.PoolPairData)
	var poolPairs []shared.PoolPairData
	var poolShare []shared.PoolShareData
	var stakePools []shared.PoolStakeData
	var poolStaking []shared.PoolStakerData
	var orderStatus []shared.LimitOrderStatus

	var poolPairsToBeDelete []shared.PoolPairData
	var poolShareToBeDelete []shared.PoolShareData
	var stakePoolsToBeDelete []shared.PoolStakeData
	var poolStakingToBeDelete []shared.PoolStakerData
	var orderStatusToBeDelete []shared.LimitOrderStatus

	for poolID, state := range statev2.PoolPairs {
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
		pairListMap[poolData.PairID] = append(pairListMap[poolData.PairID], poolData)
		for shareID, share := range state.Shares {
			tradingFee := make(map[string]uint64)
			for k, v := range share.TradingFees() {
				tradingFee[k.String()] = v
			}
			shareData := shared.PoolShareData{
				Version:    2,
				PoolID:     poolID,
				Amount:     share.Amount(),
				TradingFee: tradingFee,
				NFTID:      shareID,
			}
			poolShare = append(poolShare, shareData)
		}

		for _, order := range state.Orderbook.Orders {
			newOrder := shared.LimitOrderStatus{
				RequestTx: order.Id(),
				Left:      order.Token0Balance(),
			}
			orderStatus = append(orderStatus, newOrder)
		}
	}

	for pairID, pools := range pairListMap {
		data := shared.PairData{
			PairID:   pairID,
			TokenID1: pools[0].TokenID1,
			TokenID2: pools[0].TokenID2,
		}

		for _, v := range pools {
			data.Token1Amount += v.Token1Amount
			data.Token2Amount += v.Token2Amount
		}
		pairList = append(pairList, data)
	}

	//comparing with old state
	if prevStatev2 != nil {
		for poolID, state := range prevStatev2.PoolPairs {
			willDelete := false
			if _, ok := statev2.PoolPairs[poolID]; !ok {
				willDelete = true
			}
			if willDelete {
				poolData := shared.PoolPairData{
					Version: 2,
					PoolID:  poolID,
					PairID:  state.State.Token0ID().String() + "-" + state.State.Token1ID().String(),
				}
				poolPairsToBeDelete = append(poolPairsToBeDelete, poolData)
				for shareID, _ := range state.Shares {
					shareData := shared.PoolShareData{
						PoolID:  poolID,
						NFTID:   shareID,
						Version: 2,
					}
					poolShareToBeDelete = append(poolShareToBeDelete, shareData)
				}
				for _, order := range state.Orderbook.Orders {
					newOrder := shared.LimitOrderStatus{
						RequestTx: order.Id(),
					}
					orderStatusToBeDelete = append(orderStatusToBeDelete, newOrder)
				}
			} else {
				newState := statev2.PoolPairs[poolID]
				for shareID, _ := range state.Shares {
					willDelete := false
					if _, ok := newState.Shares[shareID]; !ok {
						willDelete = true
					}
					if willDelete {
						shareData := shared.PoolShareData{
							PoolID:  poolID,
							NFTID:   shareID,
							Version: 2,
						}
						poolShareToBeDelete = append(poolShareToBeDelete, shareData)
					}
				}
				for _, order := range state.Orderbook.Orders {
					willDelete := true
					for _, v := range newState.Orderbook.Orders {
						if v.Id() == order.Id() {
							willDelete = false
						}
					}
					if willDelete {
						newOrder := shared.LimitOrderStatus{
							RequestTx: order.Id(),
						}
						orderStatusToBeDelete = append(orderStatusToBeDelete, newOrder)
					}

				}
			}

		}
	}

	for tokenID, stakeData := range statev2.StakingPoolsState {
		poolData := shared.PoolStakeData{
			Amount:  stakeData.Liquidity(),
			TokenID: tokenID,
		}
		stakePools = append(stakePools, poolData)
		for nftID, staker := range stakeData.Stakers() {
			rewardMap := make(map[string]uint64)
			for k, v := range staker.Rewards() {
				rewardMap[k.String()] = v
			}
			stake := shared.PoolStakerData{
				TokenID: tokenID,
				NFTID:   nftID,
				Amount:  stakeData.Liquidity(),
				Reward:  rewardMap,
			}
			poolStaking = append(poolStaking, stake)
		}
	}

	for tokenID, stakeData := range prevStatev2.StakingPoolsState {
		willDelete := false
		if _, ok := statev2.StakingPoolsState[tokenID]; !ok {
			willDelete = true
		}
		if willDelete {
			poolData := shared.PoolStakeData{
				TokenID: tokenID,
			}
			stakePoolsToBeDelete = append(stakePoolsToBeDelete, poolData)
			for nftID, _ := range stakeData.Stakers() {
				stake := shared.PoolStakerData{
					TokenID: tokenID,
					NFTID:   nftID,
				}
				poolStakingToBeDelete = append(poolStakingToBeDelete, stake)
			}
		} else {
			newStaker := statev2.StakingPoolsState[tokenID].Stakers()
			for nftID, _ := range stakeData.Stakers() {
				willDelete := false
				if _, ok := newStaker[nftID]; !ok {
					willDelete = true
				}
				if willDelete {
					stake := shared.PoolStakerData{
						TokenID: tokenID,
						NFTID:   nftID,
					}
					poolStakingToBeDelete = append(poolStakingToBeDelete, stake)
				}
			}
		}
	}

	return pairList, poolPairs, poolShare, stakePools, poolStaking, orderStatus, poolPairsToBeDelete, poolShareToBeDelete, stakePoolsToBeDelete, poolStakingToBeDelete, orderStatusToBeDelete, nil
}
