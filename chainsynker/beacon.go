package chainsynker

import (
	"encoding/base64"
	"fmt"
	"log"
	"math/big"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/pdexv3/pathfinder"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/blockchain/pdex"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/config"
	"github.com/incognitochain/incognito-chain/dataaccessobject/rawdbv2"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	instruction "github.com/incognitochain/incognito-chain/instruction/pdexv3"
	"github.com/incognitochain/incognito-chain/metadata"
	metadataCommon "github.com/incognitochain/incognito-chain/metadata/common"
	metadataPdexv3 "github.com/incognitochain/incognito-chain/metadata/pdexv3"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
)

type IncPdexState struct {
	StakingPools map[string]*pdex.StakingPoolState
	PoolPairs    map[string]*pdex.PoolPairState
	Param        *statedb.Pdexv3Params
}

var pdexV3State pdex.State

func processBeacon(bc *blockchain.BlockChain, h common.Hash, height uint64) {
	log.Printf("start processing coin for block %v beacon\n", height)
	startTime := time.Now()
	var beaconBestState *blockchain.BeaconBestState
	if height < config.Param().PDexParams.Pdexv3BreakPointHeight {
		beaconBestState, _ = Localnode.GetBlockchain().GetBeaconViewStateDataFromBlockHash(h, false, false)
	} else {
		beaconBestState, _ = Localnode.GetBlockchain().GetBeaconViewStateDataFromBlockHash(h, false, true)
	}
	blk := beaconBestState.BestBlock
	beaconFeatureStateRootHash := beaconBestState.FeatureStateDBRootHash
	beaconFeatureStateDB, err := statedb.NewWithPrefixTrie(beaconFeatureStateRootHash, statedb.NewDatabaseAccessWarper(Localnode.GetBlockchain().GetBeaconChainDatabase()))
	if err != nil {
		log.Println(err)
	}
	log.Printf("beaconFeatureStateDB loaded for block %v beacon in %v\n", blk.GetHeight(), time.Since(startTime))
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
	log.Printf("beaconFeatureStateDB loaded for block %v beacon in %v\n", blk.GetHeight(), time.Since(startTime))
	// Process PDEstatev1
	if height < config.Param().PDexParams.Pdexv3BreakPointHeight {
		state := Localnode.GetBlockchain().GetBeaconBestState().PdeState(1)
		tradingFees := state.Reader().TradingFees()
		shares := state.Reader().Shares()

		waitingContributions := map[string]*rawdbv2.PDEContribution{}
		poolPairs := map[string]*rawdbv2.PDEPoolForPair{}

		err = json.Unmarshal(state.Reader().PoolPairs(), &poolPairs)
		if err != nil {
			panic(err)
		}

		err = json.Unmarshal(state.Reader().WaitingContributions(), &waitingContributions)
		if err != nil {
			panic(err)
		}

		pdeStateJSON := jsonresult.CurrentPDEState{
			BeaconTimeStamp:         beaconBestState.BestBlock.Header.Timestamp,
			PDEPoolPairs:            poolPairs,
			PDEShares:               shares,
			WaitingPDEContributions: waitingContributions,
			PDETradingFees:          tradingFees,
		}
		log.Printf("prepare pdeStateJSON for block %v beacon in %v\n", blk.GetHeight(), time.Since(startTime))
		pdeStr, err := json.MarshalToString(pdeStateJSON)
		if err != nil {
			log.Println(err)
		}
		err = database.DBSavePDEState(pdeStr, height, 1)
		if err != nil {
			log.Println(err)
		}
		log.Printf("mongo stored pdeStateJSON for block %v beacon in %v\n", blk.GetHeight(), time.Since(startTime))
	}

	//process PDEstateV2
	if beaconBestState.BeaconHeight >= config.Param().PDexParams.Pdexv3BreakPointHeight {
		var prevStateV2 *shared.PDEStateV2
		stateV2 := &shared.PDEStateV2{
			StakingPoolsState: make(map[string]*pdex.StakingPoolState),
		}
		pdeStateJSON := jsonresult.Pdexv3State{}
		var pdeStr string
		var wg sync.WaitGroup
		if height > config.Param().PDexParams.Pdexv3BreakPointHeight {
			prevStateV2 = &shared.PDEStateV2{
				StakingPoolsState: make(map[string]*pdex.StakingPoolState),
			}
			if pdexV3State == nil {
				prevBeaconFeatureStateDB, err := Localnode.GetBlockchain().GetBestStateBeaconFeatureStateDBByHeight(height-1, Localnode.GetBlockchain().GetBeaconChainDatabase())
				if err != nil {
					log.Println(err)
				}
				params, err := statedb.GetPdexv3Params(prevBeaconFeatureStateDB)
				if err != nil {
					panic(err)
				}
				for stakingPoolID := range params.StakingPoolsShare() {
					stakers, liquidity, err := initStakers(stakingPoolID, prevBeaconFeatureStateDB)
					if err != nil {
						panic(err)
					}
					rewardsPerShare, err := statedb.GetPdexv3StakingPoolRewardsPerShare(prevBeaconFeatureStateDB, stakingPoolID)
					if err != nil {
						panic(err)
					}
					prevStateV2.StakingPoolsState[stakingPoolID] = pdex.NewStakingPoolStateWithValue(liquidity, stakers, rewardsPerShare)
				}
				poolPairs, err := initPoolPairStatesFromDB(prevBeaconFeatureStateDB)
				if err != nil {
					panic(err)
				}
				pools := make(map[string]*shared.PoolPairState)
				d, err := json.Marshal(poolPairs)
				if err != nil {
					panic(err)
				}
				err = json.Unmarshal(d, &pools)
				if err != nil {
					panic(err)
				}
				prevStateV2.PoolPairs = pools
			} else {
				prevPdexv3State := pdexV3State.Clone()
				pools := make(map[string]*shared.PoolPairState)
				err = json.Unmarshal(prevPdexv3State.Reader().PoolPairs(), &pools)
				if err != nil {
					panic(err)
				}
				prevStateV2.PoolPairs = pools
				prevStateV2.StakingPoolsState = prevPdexv3State.Reader().StakingPools()
			}

		}
		if pdexV3State == nil {
			pdeStates, err := pdex.InitStatesFromDB(beaconFeatureStateDB, beaconBestState.BeaconHeight)
			if err != nil {
				panic(err)
			}
			pdexV3State = pdeStates[2]
		}

		pdeStateEnv := pdex.
			NewStateEnvBuilder().
			BuildBeaconInstructions(blk.Body.Instructions).
			BuildStateDB(beaconFeatureStateDB).
			BuildPrevBeaconHeight(blk.Header.Height - 1).
			BuildBCHeightBreakPointPrivacyV2(config.Param().BCHeightBreakPointPrivacyV2).
			BuildPdexv3BreakPoint(config.Param().PDexParams.Pdexv3BreakPointHeight).
			Build()
		err = pdexV3State.Process(pdeStateEnv)
		if err != nil {
			panic(err)
		}
		pdexV3State.ClearCache()
		params := pdexV3State.Reader().Params()
		pdexV3State.Reader().PoolPairs()
		stateV2.StakingPoolsState = pdexV3State.Reader().StakingPools()
		pools := make(map[string]*shared.PoolPairState)
		err = json.Unmarshal(pdexV3State.Reader().PoolPairs(), &pools)
		if err != nil {
			panic(err)
		}
		stateV2.PoolPairs = pools
		stateV2.Params = params

		poolPairsJSON := make(map[string]*pdex.PoolPairState)
		err = json.Unmarshal(pdexV3State.Reader().PoolPairs(), &poolPairsJSON)
		if err != nil {
			panic(err)
		}

		wg.Add(2)
		go func() {
			paramJSON := pdexV3State.Reader().Params().Clone()
			pdeStateJSON = jsonresult.Pdexv3State{
				BeaconTimeStamp: beaconBestState.BestBlock.Header.Timestamp,
				PoolPairs:       &poolPairsJSON,
				StakingPools:    &stateV2.StakingPoolsState,
				Params:          paramJSON,
			}
			pdeStr, err = json.MarshalToString(pdeStateJSON)
			if err != nil {
				log.Println(err)
			}
			wg.Done()
		}()

		var instructions []shared.InstructionBeaconData
		go func() {
			log.Printf("extractBeaconInstruction %v \n", blk.GetHeight())
			instructions, err = extractBeaconInstruction(beaconBestState.BestBlock.Body.Instructions)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
		wg.Wait()

		log.Printf("prepare state %v beacon in %v\n", blk.GetHeight(), time.Since(startTime))
		pairDatas, poolDatas, sharesDatas, poolStakeDatas, poolStakersDatas, orderBook, poolDatasToBeDel, sharesDatasToBeDel, poolStakeDatasToBeDel, poolStakersDatasToBeDel, orderBookToBeDel, rewardRecords, err := processPoolPairs(stateV2, prevStateV2, &pdeStateJSON, beaconBestState.BeaconHeight)
		if err != nil {
			panic(err)
		}

		wg.Add(14)
		go func() {
			log.Printf("done process state %v beacon in %v\n", blk.GetHeight(), time.Since(startTime))
			err = database.DBSavePDEState(pdeStr, height, 2)
			if err != nil {
				log.Println(err)
			}
			log.Printf("save pdex state %v beacon in %v\n", blk.GetHeight(), time.Since(startTime))

			wg.Done()
		}()

		go func() {
			err = database.DBSaveInstructionBeacon(instructions)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()

		go func() {
			err = database.DBSaveRewardRecord(rewardRecords)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()

		go func() {
			err = database.DBUpdatePDEPairListData(pairDatas)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
		go func() {
			err = database.DBUpdatePDEPoolInfoData(poolDatas)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
		go func() {
			err = database.DBUpdatePDEPoolShareData(sharesDatas)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
		go func() {
			err = database.DBUpdatePDEPoolStakeData(poolStakeDatas)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
		go func() {
			err = database.DBUpdatePDEPoolStakerData(poolStakersDatas)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
		go func() {
			err = database.DBUpdateOrderProgress(orderBook)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
		go func() {
			err = database.DBDeletePDEPoolData(poolDatasToBeDel)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
		go func() {
			err = database.DBDeletePDEPoolShareData(sharesDatasToBeDel)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
		go func() {
			err = database.DBDeletePDEPoolStakeData(poolStakeDatasToBeDel)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
		go func() {
			err = database.DBDeletePDEPoolStakerData(poolStakersDatasToBeDel)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
		go func() {
			err = database.DBDeleteOrderProgress(orderBookToBeDel)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
		wg.Wait()
		log.Printf("save pdex state 11 %v beacon in %v\n", blk.GetHeight(), time.Since(startTime))

	}

	statePrefix := BeaconData
	t10 := time.Now()
	err = Localnode.GetUserDatabase().Put([]byte(statePrefix), []byte(fmt.Sprintf("%v", blk.Header.Height)), nil)
	if err != nil {
		panic(err)
	}
	log.Printf("lvdb stored coin for block %v beacon in %v\n", blk.GetHeight(), time.Since(t10))
	blockProcessedLock.Lock()
	blockProcessed[-1] = blk.Header.Height
	blockProcessedLock.Unlock()
	log.Printf("finish processing coin for block %v beacon in %v\n", blk.GetHeight(), time.Since(startTime))
}

func processPoolPairs(statev2 *shared.PDEStateV2, prevStatev2 *shared.PDEStateV2, stateV2Json *jsonresult.Pdexv3State, beaconHeight uint64) ([]shared.PairInfoData, []shared.PoolInfoData, []shared.PoolShareData, []shared.PoolStakeData, []shared.PoolStakerData, []shared.LimitOrderStatus, []shared.PoolInfoData, []shared.PoolShareData, []shared.PoolStakeData, []shared.PoolStakerData, []shared.LimitOrderStatus, []shared.RewardRecord, error) {
	var pairList []shared.PairInfoData
	pairListMap := make(map[string][]shared.PoolInfoData)
	var poolPairs []shared.PoolInfoData
	var poolShare []shared.PoolShareData
	var stakePools []shared.PoolStakeData
	var poolStaking []shared.PoolStakerData
	var orderStatus []shared.LimitOrderStatus
	var rewardRecords []shared.RewardRecord

	var poolPairsToBeDelete []shared.PoolInfoData
	var poolShareToBeDelete []shared.PoolShareData
	var stakePoolsToBeDelete []shared.PoolStakeData
	var poolStakingToBeDelete []shared.PoolStakerData
	var orderStatusToBeDelete []shared.LimitOrderStatus
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		for poolID, state := range statev2.PoolPairs {
			poolData := shared.PoolInfoData{
				Version:        2,
				PoolID:         poolID,
				PairID:         state.State.Token0ID().String() + "-" + state.State.Token1ID().String(),
				AMP:            state.State.Amplifier(),
				TokenID1:       state.State.Token0ID().String(),
				TokenID2:       state.State.Token1ID().String(),
				Token1Amount:   fmt.Sprintf("%v", state.State.Token0RealAmount()),
				Token2Amount:   fmt.Sprintf("%v", state.State.Token1RealAmount()),
				Virtual1Amount: fmt.Sprintf("%v", state.State.Token0VirtualAmount().Uint64()),
				Virtual2Amount: fmt.Sprintf("%v", state.State.Token1VirtualAmount().Uint64()),
				TotalShare:     fmt.Sprintf("%v", state.State.ShareAmount()),
			}
			poolPairs = append(poolPairs, poolData)
			pairListMap[poolData.PairID] = append(pairListMap[poolData.PairID], poolData)
			for shareID, share := range state.Shares {
				tradingFee := make(map[string]uint64)
				shareIDHash, err := common.Hash{}.NewHashFromStr(shareID)
				if err != nil {
					panic(err)
				}
				rewards, err := recomputeLPFee(state.Shares, state.LpFeesPerShare, *shareIDHash)
				if err != nil {
					panic(err)
				}
				for k, v := range rewards {
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
					RequestTx:     order.Id(),
					Token1Balance: fmt.Sprintf("%v", order.Token0Balance()),
					Token2Balance: fmt.Sprintf("%v", order.Token1Balance()),
					Direction:     order.TradeDirection(),
					PoolID:        poolID,
					PairID:        poolData.PairID,
				}
				orderStatus = append(orderStatus, newOrder)
			}
		}
		for pairID, pools := range pairListMap {
			data := shared.PairInfoData{
				PairID:    pairID,
				TokenID1:  pools[0].TokenID1,
				TokenID2:  pools[0].TokenID2,
				PoolCount: len(pools),
			}
			tk1Amount := uint64(0)
			tk2Amount := uint64(0)
			for _, v := range pools {
				a1, _ := strconv.ParseUint(v.Token1Amount, 10, 64)
				a2, _ := strconv.ParseUint(v.Token2Amount, 10, 64)
				tk1Amount += a1
				tk2Amount += a2
			}
			data.Token1Amount = fmt.Sprintf("%v", tk1Amount)
			data.Token1Amount = fmt.Sprintf("%v", tk1Amount)
			pairList = append(pairList, data)
		}
		wg.Done()
	}()

	go func() {
		for tokenID, stakeData := range statev2.StakingPoolsState {
			poolData := shared.PoolStakeData{
				Amount:  stakeData.Liquidity(),
				TokenID: tokenID,
			}
			stakePools = append(stakePools, poolData)
			for shareID, staker := range stakeData.Stakers() {
				rewardMap := make(map[string]uint64)

				shareIDHash, err := common.Hash{}.NewHashFromStr(shareID)
				if err != nil {
					panic(err)
				}
				reward, err := statev2.StakingPoolsState[tokenID].RecomputeStakingRewards(*shareIDHash)
				if err != nil {
					panic(err)
				}
				for k, v := range reward {
					rewardMap[k.String()] = v
				}
				stake := shared.PoolStakerData{
					TokenID: tokenID,
					NFTID:   shareID,
					Amount:  staker.Liquidity(),
					Reward:  rewardMap,
				}
				poolStaking = append(poolStaking, stake)
			}
		}
		wg.Done()
	}()
	wg.Wait()
	for tokenID, _ := range statev2.StakingPoolsState {
		willDelete := false
		if _, ok := statev2.Params.StakingPoolsShare[tokenID]; !ok {
			willDelete = true
		}
		if willDelete {
			poolData := shared.PoolStakeData{
				TokenID: tokenID,
			}
			stakePoolsToBeDelete = append(stakePoolsToBeDelete, poolData)
		}
	}
	//comparing with old state
	if prevStatev2 != nil {
		var wg sync.WaitGroup
		var poolPairsArr []*shared.Pdexv3PoolPairWithId
		for poolId, element := range statev2.PoolPairs {

			var poolPair rawdbv2.Pdexv3PoolPair
			var poolPairWithId shared.Pdexv3PoolPairWithId

			poolPair = element.State
			poolPairWithId = shared.Pdexv3PoolPairWithId{
				poolPair,
				shared.Pdexv3PoolPairChild{
					PoolID: poolId},
			}

			poolPairsArr = append(poolPairsArr, &poolPairWithId)
		}
		wg.Add(3)
		go func() {
			for poolID, state := range statev2.StakingPoolsState {
				rw, err := extractPDEStakingReward(poolID, statev2.StakingPoolsState, prevStatev2.StakingPoolsState, beaconHeight)
				if err != nil {
					panic(err)
				}
				rewardReceive := uint64(0)
				tokenStakeAmount := state.Liquidity()

				_, receiveStake := pathfinder.FindGoodTradePath(
					5,
					poolPairsArr,
					*stateV2Json.PoolPairs,
					poolID,
					common.PRVCoinID.String(),
					tokenStakeAmount)

				for tk, v := range rw {
					_, receive := pathfinder.FindGoodTradePath(
						5,
						poolPairsArr,
						*stateV2Json.PoolPairs,
						tk,
						common.PRVCoinID.String(),
						v)
					rewardReceive += receive
				}
				var rwInfo struct {
					RewardPerToken     map[string]uint64
					TokenAmount        map[string]uint64
					RewardReceiveInPRV uint64
					TotalAmountInPRV   uint64
				}
				rwInfo.TokenAmount = make(map[string]uint64)
				rwInfo.RewardPerToken = rw
				rwInfo.TokenAmount[poolID] = tokenStakeAmount
				rwInfo.RewardReceiveInPRV = rewardReceive
				rwInfo.TotalAmountInPRV = receiveStake

				rwInfoBytes, err := json.Marshal(rwInfo)
				if err != nil {
					panic(err)
				}
				data := shared.RewardRecord{
					DataID:       poolID,
					Data:         string(rwInfoBytes),
					BeaconHeight: beaconHeight,
				}
				rewardRecords = append(rewardRecords, data)
			}
			// if beaconHeight%config.Param().EpochParam.NumberOfBlockInEpoch == 0 {
			for poolID, state := range statev2.PoolPairs {
				rw, err := extractLqReward(poolID, statev2.PoolPairs, prevStatev2.PoolPairs)
				if err != nil {
					panic(err)
				}
				rewardReceive := uint64(0)
				token1Amount := state.State.Token0RealAmount()
				token2Amount := state.State.Token1RealAmount()

				_, receive1 := pathfinder.FindGoodTradePath(
					5,
					poolPairsArr,
					*stateV2Json.PoolPairs,
					state.State.Token0ID().String(),
					common.PRVCoinID.String(),
					token1Amount)
				_, receive2 := pathfinder.FindGoodTradePath(
					5,
					poolPairsArr,
					*stateV2Json.PoolPairs,
					state.State.Token1ID().String(),
					common.PRVCoinID.String(),
					token2Amount)

				totalAmount := receive1 + receive2
				for tk, v := range rw {
					_, receive := pathfinder.FindGoodTradePath(
						5,
						poolPairsArr,
						*stateV2Json.PoolPairs,
						tk,
						common.PRVCoinID.String(),
						v)
					rewardReceive += receive
				}
				var rwInfo struct {
					RewardPerToken     map[string]uint64
					TokenAmount        map[string]uint64
					RewardReceiveInPRV uint64
					TotalAmountInPRV   uint64
				}
				rwInfo.TokenAmount = make(map[string]uint64)
				rwInfo.RewardPerToken = rw
				rwInfo.TokenAmount[state.State.Token0ID().String()] = token1Amount
				rwInfo.TokenAmount[state.State.Token1ID().String()] = token2Amount
				rwInfo.RewardReceiveInPRV = rewardReceive
				rwInfo.TotalAmountInPRV = totalAmount

				rwInfoBytes, err := json.Marshal(rwInfo)
				if err != nil {
					panic(err)
				}
				data := shared.RewardRecord{
					DataID:       poolID,
					Data:         string(rwInfoBytes),
					BeaconHeight: beaconHeight,
				}
				rewardRecords = append(rewardRecords, data)
			}
			// }
			wg.Done()
		}()

		go func() {
			for poolID, state := range prevStatev2.PoolPairs {
				willDelete := false
				if _, ok := statev2.PoolPairs[poolID]; !ok {
					willDelete = true
				}
				if willDelete {
					tkState := state.State
					poolData := shared.PoolInfoData{
						Version: 2,
						PoolID:  poolID,
						PairID:  tkState.Token0ID().String() + "-" + tkState.Token1ID().String(),
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
			wg.Done()
		}()

		go func() {
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
			wg.Done()
		}()
		wg.Wait()
	}

	return pairList, poolPairs, poolShare, stakePools, poolStaking, orderStatus, poolPairsToBeDelete, poolShareToBeDelete, stakePoolsToBeDelete, poolStakingToBeDelete, orderStatusToBeDelete, rewardRecords, nil
}

func extractBeaconInstruction(insts [][]string) ([]shared.InstructionBeaconData, error) {
	var result []shared.InstructionBeaconData
	for _, inst := range insts {
		data := shared.InstructionBeaconData{}
		if len(inst) <= 2 {
			continue
		}
		metadataType, err := strconv.Atoi(inst[0])
		if err != nil {
			continue // Not error, just not PDE instructions
		}

		data.Metatype = inst[0]
		switch metadataType {
		case metadata.PDEWithdrawalRequestMeta:
			if inst[2] == common.PDEWithdrawalAcceptedChainStatus {
				contentBytes := []byte(inst[3])
				var withdrawalRequestAction metadata.PDEWithdrawalRequestAction
				err = json.Unmarshal(contentBytes, &withdrawalRequestAction)
				if err != nil {
					panic(err)
				}
				data.Content = inst[3]
				data.Status = inst[2]
				data.TxRequest = withdrawalRequestAction.TxReqID.String()
			}

		case metadata.PDEFeeWithdrawalRequestMeta:
			contentStr := inst[3]
			contentBytes, err := base64.StdEncoding.DecodeString(contentStr)
			if err != nil {
				panic(err)
			}
			var feeWithdrawalRequestAction metadata.PDEFeeWithdrawalRequestAction
			err = json.Unmarshal(contentBytes, &feeWithdrawalRequestAction)
			if err != nil {
				panic(err)
			}
			data.Content = inst[3]
			data.Status = inst[2]
			data.TxRequest = feeWithdrawalRequestAction.TxReqID.String()
		case metadataCommon.Pdexv3WithdrawLiquidityRequestMeta:
			data.Status = inst[1]
			data.Content = inst[2]
			switch inst[1] {
			case common.PDEWithdrawalRejectedChainStatus:
				rejectWithdrawLiquidity := instruction.NewRejectWithdrawLiquidity()
				err := rejectWithdrawLiquidity.FromStringSlice(inst)
				if err != nil {
					panic(err)
				}
				data.TxRequest = rejectWithdrawLiquidity.TxReqID().String()
			case common.PDEWithdrawalAcceptedChainStatus:
				acceptWithdrawLiquidity := instruction.NewAcceptWithdrawLiquidity()
				err := acceptWithdrawLiquidity.FromStringSlice(inst)
				if err != nil {
					panic(err)
				}
				data.TxRequest = acceptWithdrawLiquidity.TxReqID().String()
			}
		// case metadataCommon.Pdexv3TradeRequestMeta:

		case metadataCommon.Pdexv3WithdrawLPFeeRequestMeta:
			data.Status = inst[2]
			data.Content = inst[3]
			var actionData metadataPdexv3.WithdrawalLPFeeContent
			err := json.Unmarshal([]byte(inst[3]), &actionData)
			if err != nil {
				panic(err)
			}
			data.TxRequest = actionData.TxReqID.String()
		// case metadataCommon.Pdexv3WithdrawProtocolFeeRequestMeta:

		// case metadataCommon.Pdexv3AddOrderRequestMeta:

		case metadataCommon.Pdexv3WithdrawOrderRequestMeta:
			data.Status = inst[1]
			data.Content = inst[2]
			switch inst[1] {
			case strconv.Itoa(metadataPdexv3.WithdrawOrderAcceptedStatus):
				currentOrder := &instruction.Action{Content: &metadataPdexv3.AcceptedWithdrawOrder{}}
				err := currentOrder.FromStringSlice(inst)
				if err != nil {
					panic(err)
				}
				data.TxRequest = currentOrder.RequestTxID().String()
			case strconv.Itoa(metadataPdexv3.WithdrawOrderRejectedStatus):
				currentOrder := &instruction.Action{Content: &metadataPdexv3.RejectedWithdrawOrder{}}
				err := currentOrder.FromStringSlice(inst)
				if err != nil {
					panic(err)
				}
				data.TxRequest = currentOrder.RequestTxID().String()
			}
		// case metadataCommon.Pdexv3DistributeStakingRewardMeta:

		case metadataCommon.Pdexv3StakingRequestMeta:
			data.Status = inst[1]
			data.Content = inst[2]
			switch inst[1] {
			case common.Pdexv3AcceptUnstakingStatus:
				acceptInst := instruction.NewAcceptStaking()
				err := acceptInst.FromStringSlice(inst)
				if err != nil {
					panic(err)
				}
				data.TxRequest = acceptInst.TxReqID().String()
			case common.Pdexv3RejectUnstakingStatus:
				rejectInst := instruction.NewRejectStaking()
				err := rejectInst.FromStringSlice(inst)
				if err != nil {
					panic(err)
				}
				data.TxRequest = rejectInst.TxReqID().String()
			}
		case metadataCommon.Pdexv3UnstakingRequestMeta:
			data.Status = inst[1]
			data.Content = inst[2]
			switch inst[1] {
			case common.Pdexv3AcceptUnstakingStatus:
				acceptInst := instruction.NewAcceptUnstaking()
				err := acceptInst.FromStringSlice(inst)
				if err != nil {
					panic(err)
				}
				data.TxRequest = acceptInst.TxReqID().String()
			case common.Pdexv3RejectUnstakingStatus:
				rejectInst := instruction.NewRejectUnstaking()
				err := rejectInst.FromStringSlice(inst)
				if err != nil {
					panic(err)
				}
				data.TxRequest = rejectInst.TxReqID().String()
			}
		case metadataCommon.Pdexv3WithdrawStakingRewardRequestMeta:
			var actionData metadataPdexv3.WithdrawalStakingRewardContent
			err := json.Unmarshal([]byte(inst[3]), &actionData)
			if err != nil {
				panic(err)
			}
			data.Status = inst[2]
			data.Content = inst[3]
			data.TxRequest = actionData.TxReqID.String()
		case metadataCommon.IssuingResponseMeta:
			//TODO: add shield respond
		default:
			continue
		}

		result = append(result, data)
	}
	return result, nil
}

func extractLqReward(poolID string, curPools map[string]*shared.PoolPairState, prevPools map[string]*shared.PoolPairState) (map[string]uint64, error) {
	result := make(map[string]uint64)

	curLPFeesPerShare, shareAmount, err := getLPFeesPerShare(poolID, curPools)
	if err != nil {
		return nil, err
	}

	oldLPFeesPerShare, _, err := getLPFeesPerShare(poolID, prevPools)
	if err != nil {
		oldLPFeesPerShare = map[common.Hash]*big.Int{}
	}

	for tokenID := range curLPFeesPerShare {
		oldFees, isExisted := oldLPFeesPerShare[tokenID]
		if !isExisted {
			oldFees = big.NewInt(0)
		}
		newFees := curLPFeesPerShare[tokenID]

		reward := new(big.Int).Mul(new(big.Int).Sub(newFees, oldFees), new(big.Int).SetUint64(shareAmount))
		reward = new(big.Int).Div(reward, pdex.BaseLPFeesPerShare)

		if !reward.IsUint64() {
			return nil, fmt.Errorf("Reward of token %v is out of range", tokenID)
		}
		if reward.Uint64() > 0 {
			result[tokenID.String()] = reward.Uint64()
		}
	}

	return result, nil
}

func getLPFeesPerShare(pairID string, pools map[string]*shared.PoolPairState) (map[common.Hash]*big.Int, uint64, error) {
	if _, ok := pools[pairID]; !ok {
		return nil, 0, fmt.Errorf("Pool pair %s not found", pairID)
	}
	pair := pools[pairID]
	return pair.LpFeesPerShare, pair.State.ShareAmount(), nil
}

func getStakingRewardsPerShare(
	stakingPoolID string, pools map[string]*pdex.StakingPoolState,
) (map[common.Hash]*big.Int, uint64, error) {

	if _, ok := pools[stakingPoolID]; !ok {
		return nil, 0, fmt.Errorf("Staking pool %s not found", stakingPoolID)
	}

	return pools[stakingPoolID].RewardsPerShare(), pools[stakingPoolID].Liquidity(), nil
}

func extractPDEStakingReward(poolID string, curPools map[string]*pdex.StakingPoolState, prevPools map[string]*pdex.StakingPoolState, beaconHeight uint64) (map[string]uint64, error) {
	result := make(map[string]uint64)

	curLPFeesPerShare, shareAmount, err := getStakingRewardsPerShare(poolID, curPools)
	if err != nil {
		return nil, err
	}

	oldLPFeesPerShare, _, err := getStakingRewardsPerShare(poolID, prevPools)
	if err != nil {
		oldLPFeesPerShare = map[common.Hash]*big.Int{}
	}

	for tokenID := range curLPFeesPerShare {
		oldFees, isExisted := oldLPFeesPerShare[tokenID]
		if !isExisted {
			oldFees = big.NewInt(0)
		}
		newFees := curLPFeesPerShare[tokenID]

		reward := new(big.Int).Mul(new(big.Int).Sub(newFees, oldFees), new(big.Int).SetUint64(shareAmount))
		reward = new(big.Int).Div(reward, pdex.BaseLPFeesPerShare)

		if !reward.IsUint64() {
			return nil, fmt.Errorf("Reward of token %v is out of range", tokenID)
		}
		if reward.Uint64() > 0 {
			result[tokenID.String()] = reward.Uint64()
		}
	}

	return result, nil
}
