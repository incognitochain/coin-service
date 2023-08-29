package chainsynker

import (
	"fmt"
	"log"
	"math"
	"math/big"
	"strings"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/pdexv3/pathfinder"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/blockchain/pdex"
	"github.com/incognitochain/incognito-chain/blockchain/types"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
)

func initStakers(stakingPoolID string, stateDB *statedb.StateDB) (map[string]*pdex.Staker, uint64, error) {
	res := make(map[string]*pdex.Staker)
	totalLiquidity := uint64(0)
	stakerStates, err := statedb.GetPdexv3Stakers(stateDB, stakingPoolID)
	if err != nil {
		return res, totalLiquidity, err
	}
	for nftID, stakerState := range stakerStates {
		totalLiquidity += stakerState.Liquidity()
		rewards, err := statedb.GetPdexv3StakerRewards(stateDB, stakingPoolID, nftID)
		if err != nil {
			return res, totalLiquidity, err
		}
		lastRewardsPerShare, err := statedb.GetPdexv3StakerLastRewardsPerShare(stateDB, stakingPoolID, nftID)
		if err != nil {
			return res, totalLiquidity, err
		}
		res[nftID] = pdex.NewStakerWithValue(stakerState.Liquidity(), rewards, lastRewardsPerShare)
	}
	return res, totalLiquidity, nil
}

func initPoolPairStatesFromDB(stateDB *statedb.StateDB) (map[string]*pdex.PoolPairState, error) {
	poolPairsStates, err := statedb.GetPdexv3PoolPairs(stateDB)
	if err != nil {
		return nil, err
	}
	res := make(map[string]*pdex.PoolPairState)
	for poolPairID, poolPairState := range poolPairsStates {
		lpFeesPerShare, err := statedb.GetPdexv3PoolPairLpFeesPerShares(stateDB, poolPairID)
		if err != nil {
			return nil, err
		}
		protocolFees, err := statedb.GetPdexv3PoolPairProtocolFees(stateDB, poolPairID)
		if err != nil {
			return nil, err
		}
		stakingPoolFees, err := statedb.GetPdexv3PoolPairStakingPoolFees(stateDB, poolPairID)
		if err != nil {
			return nil, err
		}
		tempOrderReward, err := statedb.GetPdexv3PoolPairOrderReward(stateDB, poolPairID)
		if err != nil {
			return nil, err
		}
		tempMakingVolume, err := statedb.GetPdexv3PoolPairMakingVolume(stateDB, poolPairID)
		if err != nil {
			return nil, err
		}

		makingVolume := make(map[common.Hash]*shared.MakingVolume)
		for tokenID, value := range tempMakingVolume {
			if makingVolume[tokenID] == nil {
				makingVolume[tokenID] = &shared.MakingVolume{
					Volume: make(map[string]*big.Int),
				}
			}
			for nftID, amount := range value {
				makingVolume[tokenID].Volume[nftID] = amount
			}
		}
		orderReward := make(map[string]*shared.OrderReward)
		for nftID, value := range tempOrderReward {
			if orderReward[nftID] == nil {
				orderReward[nftID] = &shared.OrderReward{
					UncollectedRewards: make(map[common.Hash]uint64),
				}
			}
			for tokenID, amount := range value {
				orderReward[nftID].UncollectedRewards[tokenID] = amount
			}
		}
		makingVolume2 := make(map[common.Hash]*pdex.MakingVolume)
		orderReward2 := make(map[string]*pdex.OrderReward)
		makingVolumeBytes, _ := json.Marshal(makingVolume)
		orderRewardBytes, _ := json.Marshal(orderReward)
		err = json.Unmarshal(makingVolumeBytes, &makingVolume2)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(orderRewardBytes, &orderReward2)
		if err != nil {
			return nil, err
		}

		shares, err := initShares(poolPairID, stateDB)
		if err != nil {
			return nil, err
		}

		orderbook := &pdex.Orderbook{}
		orderMap, err := statedb.GetPdexv3Orders(stateDB, poolPairState.PoolPairID())
		if err != nil {
			return nil, err
		}
		for _, item := range orderMap {
			v := item.Value()
			orderbook.InsertOrder(&v)
		}
		lmRewardsPerShare, err := statedb.GetPdexv3PoolPairLmRewardPerShares(stateDB, poolPairID)
		if err != nil {
			return nil, err
		}
		lmLockedShare, err := statedb.GetPdexv3PoolPairLmLockedShare(stateDB, poolPairID)
		if err != nil {
			return nil, err
		}

		poolPair := pdex.NewPoolPairStateWithValue(
			poolPairState.Value(), shares, *orderbook,
			lpFeesPerShare, lmRewardsPerShare, protocolFees, stakingPoolFees, makingVolume2, orderReward2, lmLockedShare)
		res[poolPairID] = poolPair
	}
	return res, nil
}

func initShares(poolPairID string, stateDB *statedb.StateDB) (map[string]*pdex.Share, error) {
	res := make(map[string]*pdex.Share)
	shareStates, err := statedb.GetPdexv3Shares(stateDB, poolPairID)
	if err != nil {
		return nil, err
	}
	for nftID, shareState := range shareStates {
		tradingFees, err := statedb.GetPdexv3ShareTradingFees(stateDB, poolPairID, nftID)
		if err != nil {
			return nil, err
		}
		lastLPFeesPerShare, err := statedb.GetPdexv3ShareLastLpFeesPerShare(stateDB, poolPairID, nftID)
		if err != nil {
			return nil, err
		}
		lastLmRewardsPerShare, err := statedb.GetPdexv3ShareLastLmRewardPerShare(stateDB, poolPairID, nftID)
		if err != nil {
			return nil, err
		}
		res[nftID] = pdex.NewShareWithValue(shareState.Amount(), shareState.LmLockedAmount(), tradingFees, lastLPFeesPerShare, lastLmRewardsPerShare)
	}
	return res, nil
}

func recomputeLPFee(
	shares map[string]*pdex.Share,
	lpFeesPerShare map[common.Hash]*big.Int,
	lmRewardsPerShare map[common.Hash]*big.Int,
	nftID common.Hash,
) (map[common.Hash]uint64, error) {

	curShare, ok := shares[nftID.String()]
	if !ok {
		return nil, fmt.Errorf("Share not found")
	}

	curLPFeesPerShare := lpFeesPerShare
	oldLPFeesPerShare := curShare.LastLPFeesPerShare()

	result := curShare.TradingFees()

	for tokenID := range curLPFeesPerShare {
		tradingFee, isExisted := result[tokenID]
		if !isExisted {
			tradingFee = 0
		}
		oldFees, isExisted := oldLPFeesPerShare[tokenID]
		if !isExisted {
			oldFees = big.NewInt(0)
		}
		newFees := curLPFeesPerShare[tokenID]

		reward := new(big.Int).Mul(new(big.Int).Sub(newFees, oldFees), new(big.Int).SetUint64(curShare.Amount()))
		reward = new(big.Int).Div(reward, pdex.BaseLPFeesPerShare)
		reward = new(big.Int).Add(reward, new(big.Int).SetUint64(tradingFee))

		if !reward.IsUint64() {
			return nil, fmt.Errorf("Reward of token %v is out of range", tokenID)
		}
		if reward.Uint64() > 0 {
			result[tokenID] = reward.Uint64()
		}
	}

	curLMRewardsPerShare := lmRewardsPerShare
	oldLMRewardsPerShare := curShare.LastLmRewardsPerShare()

	for tokenID := range curLMRewardsPerShare {
		tradingFee, isExisted := result[tokenID]
		if !isExisted {
			tradingFee = 0
		}
		oldFees, isExisted := oldLMRewardsPerShare[tokenID]
		if !isExisted {
			oldFees = big.NewInt(0)
		}
		newFees := curLMRewardsPerShare[tokenID]

		reward := new(big.Int).Mul(new(big.Int).Sub(newFees, oldFees), new(big.Int).SetUint64(curShare.Amount()-curShare.LmLockedShareAmount()))
		reward = new(big.Int).Div(reward, pdex.BaseLPFeesPerShare)
		reward = new(big.Int).Add(reward, new(big.Int).SetUint64(tradingFee))

		if !reward.IsUint64() {
			return nil, fmt.Errorf("Reward of token %v is out of range", tokenID)
		}
		if reward.Uint64() > 0 {
			result[tokenID] = reward.Uint64()
		}
	}
	return result, nil
}

func getRateMinimum(tokenID1, tokenID2 string, minAmount uint64, pools []*shared.Pdexv3PoolPairWithId, poolPairStates map[string]*pdex.PoolPairState, feeRateBPS uint) float64 {
	a := uint64(minAmount)
	a1 := uint64(0)
retry:
	_, receive := pathfinder.FindGoodTradePath(
		3,
		pools,
		poolPairStates,
		tokenID1,
		tokenID2,
		a, feeRateBPS)

	if receive == 0 {
		a *= 10
		if a < 1e6 {
			goto retry
		}
		return 0
	} else {
		if receive > a1*10 {
			a *= 10
			a1 = receive
			goto retry
		} else {
			if receive < a1*10 {
				a /= 10
				receive = a1
				fmt.Println("receive", a, receive)
			}
		}
	}
	return float64(receive) / float64(a)
}

func getRateMinimum2(tokenID1, tokenID2 string, minAmount uint64, pools []*shared.Pdexv3PoolPairWithId, poolPairStates map[string]*pdex.PoolPairState, feeRateBPS uint) float64 {
	a := uint64(minAmount)
	// 	a1 := uint64(0)
	// retry:
	_, receive := pathfinder.FindGoodTradePath(
		3,
		pools,
		poolPairStates,
		tokenID1,
		tokenID2,
		a, feeRateBPS)

	// if receive == 0 {
	// 	a *= 10
	// 	if a < 1e6 {
	// 		goto retry
	// 	}
	// 	return 0
	// } else {
	// 	if receive > a1*10 {
	// 		a *= 10
	// 		a1 = receive
	// 		goto retry
	// 	} else {
	// 		if receive < a1*10 {
	// 			a /= 10
	// 			receive = a1
	// 			fmt.Println("receive", a, receive)
	// 		}
	// 	}
	// }
	return float64(receive) / float64(a)
}

func calcRateSimple(virtA, virtB float64) float64 {
	return virtB / virtA
}

func getPRVPrice(defaultPools map[string]*pdex.PoolPairState, baseToken string) float64 {
	var result float64
	poolAmountTk := uint64(0)
	poolAmountTkBase := uint64(0)
retry:
	dcrate, _, _, err := getPdecimalRate(common.PRVCoinID.String(), baseToken)
	if err != nil {
		log.Println("getPdecimalRate", err)
		goto retry
	}
	for poolID, poolData := range defaultPools {
		if strings.Contains(poolID, common.PRVCoinID.String()) && strings.Contains(poolID, baseToken) {
			state := poolData.State()

			tks := strings.Split(poolID, "-")
			if tks[0] == baseToken {
				tk1Amount := state.Token1RealAmount()
				tk2Amount := state.Token0RealAmount()
				if tk1Amount == 0 || tk2Amount == 0 {
					continue
				}
				if tk2Amount < poolAmountTk && tk1Amount < poolAmountTkBase {
					continue
				}
				poolAmountTk = tk2Amount
				poolAmountTkBase = tk1Amount
				tk1VA := state.Token1VirtualAmount()
				tk2VA := state.Token0VirtualAmount()
				result = calcRateSimple(float64(tk1VA.Uint64()), float64(tk2VA.Uint64()))
			} else {
				tk1Amount := state.Token0RealAmount()
				tk2Amount := state.Token1RealAmount()
				if tk1Amount == 0 || tk2Amount == 0 {
					continue
				}
				if tk1Amount < poolAmountTk && tk2Amount < poolAmountTkBase {
					continue
				}
				poolAmountTk = tk1Amount
				poolAmountTkBase = tk2Amount
				tk1VA := state.Token0VirtualAmount()
				tk2VA := state.Token1VirtualAmount()
				result = calcRateSimple(float64(tk1VA.Uint64()), float64(tk2VA.Uint64()))
			}
		}
	}
	return result * dcrate
}

func getPdecimalRate(tokenID1, tokenID2 string) (float64, int, int, error) {
	tk1Decimal := 0
	tk2Decimal := 0
	tk1, err := database.DBGetExtraTokenInfo(tokenID1)
	if err != nil {
		log.Println(err)
	}
	tk2, err := database.DBGetExtraTokenInfo(tokenID2)
	if err != nil {
		log.Println(err)
	}
	if tk1 != nil {
		tk1Decimal = int(tk1.PDecimals)
	}
	if tk2 != nil {
		tk2Decimal = int(tk2.PDecimals)
	}
	result := math.Pow10(tk1Decimal) / math.Pow10(tk2Decimal)
	return result, tk1Decimal, tk2Decimal, nil
}

func extractInstructionFromBeaconBlocks(shardPrevBlkHash []byte, currentBeaconHeight uint64) ([][]string, error) {
	var blk types.ShardBlock
	blkBytes, err := Localnode.GetUserDatabase().Get(shardPrevBlkHash, nil)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(blkBytes, &blk); err != nil {
		panic(err)
	}
	var insts [][]string
	for i := blk.Header.BeaconHeight + 1; i <= currentBeaconHeight; i++ {
		bblk, err := Localnode.GetBlockchain().GetBeaconBlockByHeight(i)
		if err != nil {
			return nil, err
		}
		insts = append(insts, bblk[0].Body.Instructions...)
	}
	return insts, nil
}
