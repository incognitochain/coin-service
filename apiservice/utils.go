package apiservice

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/pdexv3/analyticsquery"
	"github.com/incognitochain/coin-service/pdexv3/pathfinder"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/blockchain/pdex"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/metadata"
	pdexv3Meta "github.com/incognitochain/incognito-chain/metadata/pdexv3"
	"github.com/incognitochain/incognito-chain/transaction"
	"github.com/incognitochain/incognito-chain/wallet"
)

func buildGinErrorRespond(err error) *APIRespond {
	errStr := err.Error()
	respond := APIRespond{
		Result: nil,
		Error:  &errStr,
	}
	return &respond
}

func buildTxDetailRespond(txDataList []shared.TxData, isBase58 bool) ([]ReceivedTransactionV2, error) {
	var wg sync.WaitGroup
	collectCh := make(chan ReceivedTransactionV2, 200)
	var result []ReceivedTransactionV2
	var errD error
	for idx, txData := range txDataList {
		wg.Add(1)
		go func(txd shared.TxData) {
			var tx metadata.Transaction
			var parseErr error
			var txChoice *transaction.TxChoice
			txChoice, parseErr = shared.DeserializeTransactionJSON([]byte(txd.TxDetail))
			if parseErr != nil {
				errD = parseErr
				return
			}
			tx = txChoice.ToTx()
			if tx == nil {
				errD = errors.New("invalid tx detected")
				return
			}
			txDetail, err := shared.NewTransactionDetail(tx, nil, txd.BlockHeight, 0, byte(txd.ShardID), isBase58)
			if err != nil {
				errD = err
				return
			}
			txDetail.BlockHash = txd.BlockHash
			txDetail.IsInBlock = true
			txDetail.Proof = nil
			txDetail.Sig = ""
			txDetail.PrivacyCustomTokenID = txd.RealTokenID
			txReceive := ReceivedTransactionV2{
				TxDetail:    txDetail,
				FromShardID: txDetail.ShardID,
			}
			collectCh <- txReceive
			wg.Done()
		}(txData)
		if (idx+1)%200 == 0 || idx+1 == len(txDataList) {
			wg.Wait()
			close(collectCh)
			for txjson := range collectCh {
				result = append(result, txjson)
			}
			collectCh = make(chan ReceivedTransactionV2, 200)
		}
	}
	return result, errD
}

func getTradeStatus(order *shared.TradeOrderData, limitOrderStatus *shared.LimitOrderStatus) (uint64, uint64, uint64, uint64, uint64, int, string, map[string]TradeWithdrawInfo, bool, error) {
	var matchedAmount uint64
	var status string
	var sellTokenWDAmount uint64
	var buyTokenWDAmount uint64
	var sellTokenBalance uint64
	var buyTokenBalance uint64
	var isCompleted bool
	statusCode := 0
	withdrawTxs := make(map[string]TradeWithdrawInfo)
	orderAmount, _ := strconv.ParseUint(order.Amount, 10, 64)
	if order.IsSwap {
		switch order.Status {
		case 0:
			status = "ongoing"
		case 1:
			status = "done"
			matchedAmount = orderAmount
			isCompleted = true
		case 2:
			status = "rejected"
			isCompleted = true
		}
		return matchedAmount, 0, 0, 0, 0, order.Status, status, nil, isCompleted, nil

	}

	if len(order.RespondTxs) > 0 {
		status = "rejected"
		isCompleted = true
	} else {
		if limitOrderStatus == nil && len(order.WithdrawInfos) == 0 {
			isCompleted = false
			orderAmount = 0
		}

		if limitOrderStatus != nil {
			if limitOrderStatus.Direction == 0 {
				sellTokenBalance, _ = strconv.ParseUint(limitOrderStatus.Token1Balance, 10, 64)
				buyTokenBalance, _ = strconv.ParseUint(limitOrderStatus.Token2Balance, 10, 64)
			} else {
				sellTokenBalance, _ = strconv.ParseUint(limitOrderStatus.Token2Balance, 10, 64)
				buyTokenBalance, _ = strconv.ParseUint(limitOrderStatus.Token1Balance, 10, 64)
			}
		}

		for wdRQtx, v := range order.WithdrawInfos {
			data := TradeWithdrawInfo{
				TokenIDs: v.TokenIDs,
				Responds: make(map[string]struct {
					Amount    uint64
					Status    int
					RespondTx string
				}),
				IsRejected: v.IsRejected,
			}
			if !v.IsRejected {
				for idx, d := range v.RespondTokens {
					rp := data.Responds[d]
					rp.Amount = v.RespondAmount[idx]
					rp.RespondTx = v.Responds[idx]
					rp.Status = v.Status[idx]
					data.Responds[d] = rp
					if d == order.SellTokenID {
						sellTokenWDAmount += rp.Amount
					}
					if d == order.BuyTokenID {
						buyTokenWDAmount += rp.Amount
					}
				}
			}

			if len(v.RespondTokens) == 0 && !v.IsRejected {
				status = "withdrawing"
			}
			withdrawTxs[wdRQtx] = data
		}
		if sellTokenBalance == 0 && buyTokenBalance == 0 && len(order.WithdrawInfos) > 0 {
			isCompleted = true
		}

		if len(order.WithdrawInfos) == 1 {
			if _, ok := order.WithdrawInfos[order.RequestTx]; ok {
				isCompleted = true
			}
		}

		matchedAmount = orderAmount - sellTokenBalance - sellTokenWDAmount
		if isCompleted {
			status = "done"
		} else {
			status = "ongoing"
		}
	}

	switch status {
	case "ongoing":
		statusCode = 0
	case "done":
		statusCode = 1
	case "rejected":
		statusCode = 2
	case "withdrawing":
		statusCode = 3
	}
	return matchedAmount, sellTokenBalance, buyTokenBalance, sellTokenWDAmount, buyTokenWDAmount, statusCode, status, withdrawTxs, isCompleted, nil
}

func extractPubkeyFromKey(key string, otakeyOnly bool) (string, error) {
	var result string
	pubkey := []byte{}
	if key == "" {
		return result, errors.New("key can't be empty")
	}
	wl, err := wallet.Base58CheckDeserialize(key)
	if err != nil {
		return result, err
	}
	if wl.KeySet.OTAKey.GetPublicSpend() == nil {
		if otakeyOnly {
			return result, errors.New("key incorrect format")
		}
		if wl.KeySet.PaymentAddress.GetPublicSpend() == nil {
			return result, errors.New("key incorrect format")
		} else {
			pubkey = wl.KeySet.PaymentAddress.GetPublicSpend().ToBytesS()
		}
	} else {
		pubkey = wl.KeySet.OTAKey.GetPublicSpend().ToBytesS()
	}

	result = base58.EncodeCheck(pubkey)
	return result, nil
}

// func calcAMPRate(virtA, virtB, sellAmount float64) float64 {
// 	var result float64
// 	k := virtA * virtB
// 	result = virtB - (k / (virtA + sellAmount))
// 	return result / sellAmount
// }

func calcRateSimple(virtA, virtB float64) float64 {
	return virtB / virtA
}

func willSwapTokenPlace(token1ID, token2ID string, tokenPriorityList []string) bool {
	token1Idxs := -1
	token2Idxs := -1
	for idx, v := range tokenPriorityList {
		if token1ID == v {
			token1Idxs = idx
		}
	}
	for idx, v := range tokenPriorityList {
		if token2ID == v {
			token2Idxs = idx
		}
	}
	return token1Idxs > token2Idxs
}

func getPoolAmount(poolID string, buyTokenID string) uint64 {
	datas, err := database.DBGetPoolPairsByPoolID([]string{poolID})
	if err != nil {
		fmt.Println("poolID cant get", poolID)
		return 0
	}
	if len(datas) > 0 {
		if datas[0].TokenID1 == buyTokenID {
			result, _ := strconv.ParseUint(datas[0].Token1Amount, 10, 64)
			return result
		} else {
			result, _ := strconv.ParseUint(datas[0].Token2Amount, 10, 64)
			return result
		}
	}
	fmt.Println("poolID amount is zero", poolID)
	return 0
}

func getRate(tokenID1, tokenID2 string, pools []*shared.Pdexv3PoolPairWithId, poolPairStates map[string]*pdex.PoolPairState) float64 {
	a := uint64(1)
	a1 := uint64(0)
retry:
	_, receive := pathfinder.FindGoodTradePath(
		pdexv3Meta.MaxTradePathLength,
		pools,
		poolPairStates,
		tokenID1,
		tokenID2,
		a)

	if receive == 0 {
		a *= 10
		if a < 1e18 {
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

func getRateMinimum(tokenID1, tokenID2 string, minAmount uint64, pools []*shared.Pdexv3PoolPairWithId, poolPairStates map[string]*pdex.PoolPairState) float64 {
	a := uint64(minAmount)
	a1 := uint64(0)
retry:
	_, receive := pathfinder.FindGoodTradePath(
		pdexv3Meta.MaxTradePathLength,
		pools,
		poolPairStates,
		tokenID1,
		tokenID2,
		a)

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

func ampHardCode(tokenID1, tokenID2 string) float64 {
	if strings.Contains(pair1, tokenID1) && strings.Contains(pair1, tokenID2) {
		return 3
	}
	if strings.Contains(pair2, tokenID1) && strings.Contains(pair2, tokenID2) {
		return 3
	}
	if strings.Contains(pair3, tokenID1) && strings.Contains(pair3, tokenID2) {
		return 3
	}
	if strings.Contains(pair4, tokenID1) && strings.Contains(pair4, tokenID2) {
		return 3
	}
	if strings.Contains(pair5, tokenID1) && strings.Contains(pair5, tokenID2) {
		return 3
	}
	if strings.Contains(pair6, tokenID1) && strings.Contains(pair6, tokenID2) {
		return 3
	}
	if strings.Contains(pair7, tokenID1) && strings.Contains(pair7, tokenID2) {
		return 2.2
	}
	if strings.Contains(pair8, tokenID1) && strings.Contains(pair8, tokenID2) {
		return 3
	}
	if strings.Contains(pair9, tokenID1) && strings.Contains(pair9, tokenID2) {
		return 2.5
	}
	if strings.Contains(pair10, tokenID1) && strings.Contains(pair10, tokenID2) {
		return 2
	}
	if strings.Contains(pair11, tokenID1) && strings.Contains(pair11, tokenID2) {
		return 2.5
	}
	if strings.Contains(pair12, tokenID1) && strings.Contains(pair12, tokenID2) {
		return 100
	}
	if strings.Contains(pair13, tokenID1) && strings.Contains(pair13, tokenID2) {
		return 100
	}
	if strings.Contains(pair14, tokenID1) && strings.Contains(pair14, tokenID2) {
		return 100
	}
	if strings.Contains(pair15, tokenID1) && strings.Contains(pair15, tokenID2) {
		return 3
	}
	if strings.Contains(pair16, tokenID1) && strings.Contains(pair16, tokenID2) {
		return 2
	}
	return 0
}

var (
	pair1  = common.PRVCoinID.String() + "b832e5d3b1f01a4f0623f7fe91d6673461e1f5d37d91fe78c5c2e6183ff39696"
	pair2  = common.PRVCoinID.String() + "ffd8d42dc40a8d166ea4848baf8b5f6e912ad79875f4373070b59392b1756c8f"
	pair3  = common.PRVCoinID.String() + "c01e7dc1d1aba995c19b257412340b057f8ad1482ccb6a9bb0adce61afbf05d4"
	pair4  = common.PRVCoinID.String() + "7450ad98cb8c967afb76503944ab30b4ce3560ed8f3acc3155f687641ae34135"
	pair5  = common.PRVCoinID.String() + "447b088f1c2a8e08bff622ef43a477e98af22b64ea34f99278f4b550d285fbff"
	pair6  = common.PRVCoinID.String() + "a609150120c0247407e6d7725f2a9701dcbb7bab5337a70b9cef801f34bc2b5c"
	pair7  = common.PRVCoinID.String() + "716fd1009e2a1669caacc36891e707bfdf02590f96ebd897548e8963c95ebac0"
	pair8  = "b832e5d3b1f01a4f0623f7fe91d6673461e1f5d37d91fe78c5c2e6183ff39696" + "716fd1009e2a1669caacc36891e707bfdf02590f96ebd897548e8963c95ebac0"
	pair9  = "ffd8d42dc40a8d166ea4848baf8b5f6e912ad79875f4373070b59392b1756c8f" + "716fd1009e2a1669caacc36891e707bfdf02590f96ebd897548e8963c95ebac0"
	pair10 = "c01e7dc1d1aba995c19b257412340b057f8ad1482ccb6a9bb0adce61afbf05d4" + "716fd1009e2a1669caacc36891e707bfdf02590f96ebd897548e8963c95ebac0"
	pair11 = "e5032c083f0da67ca141331b6005e4a3740c50218f151a5e829e9d03227e33e2" + "716fd1009e2a1669caacc36891e707bfdf02590f96ebd897548e8963c95ebac0"
	pair12 = "1ff2da446abfebea3ba30385e2ca99b0f0bbeda5c6371f4c23c939672b429a42" + "716fd1009e2a1669caacc36891e707bfdf02590f96ebd897548e8963c95ebac0"
	pair13 = "3f89c75324b46f13c7b036871060e641d996a24c09b3065835cb1d38b799d6c1" + "716fd1009e2a1669caacc36891e707bfdf02590f96ebd897548e8963c95ebac0"
	pair14 = "be02b225bcd26eeae00d3a51e554ac0adcdcc09de77ad03202904666d427a7e4" + "716fd1009e2a1669caacc36891e707bfdf02590f96ebd897548e8963c95ebac0"
	pair15 = common.PRVCoinID.String() + "e5032c083f0da67ca141331b6005e4a3740c50218f151a5e829e9d03227e33e2"
	pair16 = common.PRVCoinID.String() + "dae027b21d8d57114da11209dce8eeb587d01adf59d4fc356a8be5eedc146859"
)

func getUniqueIdx(list []string) []int {
	unique := make(map[string]int)
	result := []int{}
	for idx, v := range list {
		if _, ok := unique[v]; !ok {
			unique[v] = idx
			result = append(result, idx)
		}
	}
	return result
}

func getToken24hPriceChange(tokenID, pairTokenID, poolPair, baseToken string, prv24hChange float64) float64 {
	if pairTokenID == baseToken {
		return getPoolPair24hChange(poolPair)
	}
	if pairTokenID == common.PRVCoinID.String() {
		return getPoolPair24hChange(poolPair) + prv24hChange
	}
	return 0
}

func getPoolPair24hChange(poolID string) float64 {
	analyticsData, err := analyticsquery.APIGetPDexV3PairRateHistories(poolID, "PT15M", "PT24H")
	if err != nil {
		log.Println(err)
		return 0
	}
	if len(analyticsData.Result) == 0 {
		return 0
	}
	p1 := analyticsData.Result[0].Close
	p2 := analyticsData.Result[len(analyticsData.Result)-1].Close
	r := (p2 - p1) / p1 * 100
	return r
}
func getTokenRoute(sellToken string, route []string) []string {
	tokenRoute := []string{sellToken}
	intermediateToken := sellToken
	for _, poolID := range route {
		tks := strings.Split(poolID, "-")
		if tks[0] != intermediateToken {
			tokenRoute = append(tokenRoute, tks[0])
			intermediateToken = tks[0]
		} else {
			tokenRoute = append(tokenRoute, tks[1])
			intermediateToken = tks[1]
		}
	}
	return tokenRoute
}

var estokenList []TokenInfo
var alltokenList []TokenInfo
var marketTokenList []TokenInfo
var tokenListLock sync.RWMutex

func getTokenList(isAll bool) []TokenInfo {
	tokenListLock.RLock()
	defer tokenListLock.RUnlock()
	newList := []TokenInfo{}
	if isAll {
		newList = append(newList, alltokenList...)
	} else {
		newList = append(newList, estokenList...)
	}
	return newList
}
func tokenListWatcher() {
	for {
		go retrieveTokenList(true)
		go retrieveTokenList(false)
		go retrieveMarketTokenList()
		time.Sleep(20 * time.Second)
	}
}

func retrieveTokenList(isAll bool) {
	startTime := time.Now()
	var wg sync.WaitGroup
	wg.Add(4)

	var extraTokenInfo []shared.ExtraTokenInfo
	var customTokenInfo []shared.CustomTokenInfo
	var defaultPools map[string]struct{}
	var priorityTokens []string
	var datalist []TokenInfo

	go func() {
		defer wg.Done()
		var err error
		extraTokenInfo, err = database.DBGetAllExtraTokenInfo()
		if err != nil {
			log.Println(err)
			return
		}
	}()

	go func() {
		defer wg.Done()
		var err error
		if isAll {
			customTokenInfo, err = database.DBGetAllCustomTokenInfo(false)
		} else {
			customTokenInfo, err = database.DBGetAllCustomTokenInfo(true)
		}
		if err != nil {
			log.Println(err)
			return
		}
	}()

	go func() {
		var err error
		defer wg.Done()
		defaultPools, err = database.DBGetDefaultPool(true)
		if err != nil {
			log.Println(err)
			return
		}
	}()
	go func() {
		var err error
		defer wg.Done()
		priorityTokens, err = database.DBGetTokenPriority()
		if err != nil {
			log.Println(err)
			return
		}
	}()
	wg.Wait()

	totalTokenList := []string{}
	extraTokenInfoMap := make(map[string]shared.ExtraTokenInfo)
	for _, v := range extraTokenInfo {
		extraTokenInfoMap[v.TokenID] = v
		totalTokenList = append(totalTokenList, v.TokenID)
	}

	customTokenInfoMap := make(map[string]shared.CustomTokenInfo)
	for _, v := range customTokenInfo {
		customTokenInfoMap[v.TokenID] = v
		totalTokenList = append(totalTokenList, v.TokenID)
	}
	chainTkListMap := make(map[string]struct{})

	baseToken, _ := database.DBGetBasePriceToken()

	prvUsdtPair24h := float64(0)
	for v, _ := range defaultPools {
		if strings.Contains(v, baseToken) && strings.Contains(v, common.PRVCoinID.String()) {
			prvUsdtPair24h = getPoolPair24hChange(v)
			break
		}
	}
	var err error
	var tokenList []shared.TokenInfoData
	if !isAll {
		tokenList, err = database.DBGetTokenByTokenID(totalTokenList)
		if err != nil {
			log.Println(err)
			return
		}
	} else {
		tokenList, err = database.DBGetAllTokenInfo()
		if err != nil {
			log.Println(err)
			return
		}
	}
	fmt.Println("APIGetTokenList1", time.Since(startTime))
	for _, v := range tokenList {
		chainTkListMap[v.TokenID] = struct{}{}
		currPrice, _ := strconv.ParseFloat(v.CurrentPrice, 64)
		pastPrice, _ := strconv.ParseFloat(v.PastPrice, 64)
		percent24h := float64(0)
		if pastPrice != 0 && currPrice != 0 {
			percent24h = ((currPrice - pastPrice) / pastPrice) * 100
		}
		data := TokenInfo{
			TokenID:          v.TokenID,
			Name:             v.Name,
			Symbol:           v.Symbol,
			IsPrivacy:        v.IsPrivacy,
			IsBridge:         v.IsBridge,
			ExternalID:       v.ExternalID,
			PriceUsd:         currPrice,
			PercentChange24h: fmt.Sprintf("%.2f", percent24h),
		}

		defaultPool := ""
		defaultPairToken := ""
		defaultPairTokenIdx := -1
		currentPoolAmount := uint64(0)
		for poolID, _ := range defaultPools {
			if strings.Contains(poolID, data.TokenID) {
				pa := getPoolAmount(poolID, data.TokenID)
				if pa == 0 {
					continue
				}
				tks := strings.Split(poolID, "-")
				tkPair := tks[0]
				if tks[0] == data.TokenID {
					tkPair = tks[1]
				}
				for idx, ptk := range priorityTokens {
					if (ptk == tkPair) && (idx >= defaultPairTokenIdx) {
						if idx > defaultPairTokenIdx {
							defaultPool = poolID
							defaultPairToken = tkPair
							defaultPairTokenIdx = idx
							currentPoolAmount = pa
						}
						if (idx == defaultPairTokenIdx) && (pa > currentPoolAmount) {
							defaultPool = poolID
							defaultPairToken = tkPair
							defaultPairTokenIdx = idx
							currentPoolAmount = pa
						}
					}
				}

				if defaultPool == "" {
					if pa > 0 {
						defaultPool = poolID
						defaultPairToken = tkPair
						currentPoolAmount = pa
					}
				} else {
					if (pa > currentPoolAmount) && (defaultPairTokenIdx == -1) {
						defaultPool = poolID
						defaultPairToken = tkPair
						currentPoolAmount = pa
					}
				}
			}
		}
		data.DefaultPairToken = defaultPairToken
		data.DefaultPoolPair = defaultPool
		if data.TokenID == common.PRVCoinID.String() {
			data.PercentChange24h = fmt.Sprintf("%.2f", prvUsdtPair24h)
		} else {
			if data.DefaultPairToken != "" && data.TokenID != baseToken {
				data.PercentChange24h = fmt.Sprintf("%.2f", getToken24hPriceChange(data.TokenID, data.DefaultPairToken, data.DefaultPoolPair, baseToken, prvUsdtPair24h))
			}
		}
		if etki, ok := customTokenInfoMap[v.TokenID]; ok {
			if etki.Name != "" {
				data.Name = etki.Name
			}
			if etki.Symbol != "" {
				data.Symbol = etki.Symbol
			}
			if etki.Verified {
				data.Verified = etki.Verified
			}
			if etki.Image != "" {
				data.Image = etki.Image
			}
		}
		if etki, ok := extraTokenInfoMap[v.TokenID]; ok {
			if etki.Name != "" {
				data.Name = etki.Name
			}
			data.Decimals = etki.Decimals
			if etki.Symbol != "" {
				data.Symbol = etki.Symbol
			}
			data.PSymbol = etki.PSymbol
			data.PDecimals = int(etki.PDecimals)
			data.ContractID = etki.ContractID
			data.Status = etki.Status
			data.Type = etki.Type
			data.CurrencyType = etki.CurrencyType
			data.Default = etki.Default
			if etki.Verified {
				data.Verified = etki.Verified
			}
			data.UserID = etki.UserID
			data.PercentChange1h = etki.PercentChange1h
			data.PercentChangePrv1h = etki.PercentChangePrv1h
			data.CurrentPrvPool = etki.CurrentPrvPool
			data.PricePrv = etki.PricePrv
			data.Volume24 = etki.Volume24
			data.ParentID = etki.ParentID
			data.OriginalSymbol = etki.OriginalSymbol
			data.LiquidityReward = etki.LiquidityReward
			data.Network = etki.Network
			err = json.UnmarshalFromString(etki.ListChildToken, &data.ListChildToken)
			if err != nil {
				panic(err)
			}
			if data.PriceUsd == 0 {
				data.PriceUsd = etki.PriceUsd
			}
		}
		if !v.IsNFT {
			datalist = append(datalist, data)
		}
	}

	fmt.Println("APIGetTokenList2", time.Since(startTime))
	for _, tkInfo := range extraTokenInfo {
		if _, ok := chainTkListMap[tkInfo.TokenID]; !ok {
			tkdata := TokenInfo{
				TokenID:      tkInfo.TokenID,
				Name:         tkInfo.Name,
				Symbol:       tkInfo.Symbol,
				PSymbol:      tkInfo.PSymbol,
				PDecimals:    int(tkInfo.PDecimals),
				Decimals:     tkInfo.Decimals,
				ContractID:   tkInfo.ContractID,
				Status:       tkInfo.Status,
				Type:         tkInfo.Type,
				CurrencyType: tkInfo.CurrencyType,
				Default:      tkInfo.Default,
				Verified:     tkInfo.Verified,
				UserID:       tkInfo.UserID,

				PriceUsd:           tkInfo.PriceUsd,
				PercentChange1h:    tkInfo.PercentChange1h,
				PercentChangePrv1h: tkInfo.PercentChangePrv1h,
				CurrentPrvPool:     tkInfo.CurrentPrvPool,
				PricePrv:           tkInfo.PricePrv,
				Volume24:           tkInfo.Volume24,
				ParentID:           tkInfo.ParentID,
				OriginalSymbol:     tkInfo.OriginalSymbol,
				LiquidityReward:    tkInfo.LiquidityReward,

				Network: tkInfo.Network,
			}
			err = json.UnmarshalFromString(tkInfo.ListChildToken, &tkdata.ListChildToken)
			if err != nil {
				panic(err)
			}
			datalist = append(datalist, tkdata)
		}
	}

	tokenListLock.Lock()
	defer tokenListLock.Unlock()
	if isAll {
		alltokenList = datalist
	} else {
		estokenList = datalist
	}
}

func retrieveMarketTokenList() {
	var datalist []TokenInfo

	defaultPools, err := database.DBGetDefaultPool(true)
	if err != nil {
		return
	}
	priorityTokens, err := database.DBGetTokenPriority()
	if err != nil {
		return
	}
	tokenMap := make(map[string]struct{})
	tokens := []string{}
	for poolID := range defaultPools {
		tks := strings.Split(poolID, "-")
		tokenMap[tks[0]] = struct{}{}
		tokenMap[tks[1]] = struct{}{}
	}
	for v := range tokenMap {
		tokens = append(tokens, v)
	}
	extraTokenInfo, err := database.DBGetExtraTokenInfoByTokenID(tokens)
	if err != nil {
		log.Println(err)
		return
	}
	customTokenInfo, err := database.DBGetCustomTokenInfoByTokenID(tokens)
	if err != nil {
		log.Println(err)
		return
	}

	extraTokenInfoMap := make(map[string]shared.ExtraTokenInfo)
	for _, v := range extraTokenInfo {
		extraTokenInfoMap[v.TokenID] = v
	}

	customTokenInfoMap := make(map[string]shared.CustomTokenInfo)
	for _, v := range customTokenInfo {
		customTokenInfoMap[v.TokenID] = v
	}
	chainTkListMap := make(map[string]struct{})

	baseToken, _ := database.DBGetBasePriceToken()

	prvUsdtPair24h := float64(0)
	for v, _ := range defaultPools {
		if strings.Contains(v, baseToken) && strings.Contains(v, common.PRVCoinID.String()) {
			prvUsdtPair24h = getPoolPair24hChange(v)
			break
		}
	}

	tokenList, err := database.DBGetTokenByTokenID(tokens)
	if err != nil {
		log.Println(err)
		return
	}

	for _, v := range tokenList {
		chainTkListMap[v.TokenID] = struct{}{}
		currPrice, _ := strconv.ParseFloat(v.CurrentPrice, 64)
		pastPrice, _ := strconv.ParseFloat(v.PastPrice, 64)
		percent24h := float64(0)
		if pastPrice != 0 && currPrice != 0 {
			percent24h = ((currPrice - pastPrice) / pastPrice) * 100
		}
		data := TokenInfo{
			TokenID:          v.TokenID,
			Name:             v.Name,
			Symbol:           v.Symbol,
			Image:            v.Image,
			IsPrivacy:        v.IsPrivacy,
			IsBridge:         v.IsBridge,
			ExternalID:       v.ExternalID,
			PriceUsd:         currPrice,
			PercentChange24h: fmt.Sprintf("%.2f", percent24h),
		}

		defaultPool := ""
		defaultPairToken := ""
		defaultPairTokenIdx := -1
		currentPoolAmount := uint64(0)
		for poolID, _ := range defaultPools {
			if strings.Contains(poolID, data.TokenID) {
				pa := getPoolAmount(poolID, data.TokenID)
				if pa == 0 {
					continue
				}
				tks := strings.Split(poolID, "-")
				tkPair := tks[0]
				if tks[0] == data.TokenID {
					tkPair = tks[1]
				}
				for idx, ptk := range priorityTokens {
					if (ptk == tkPair) && (idx >= defaultPairTokenIdx) {
						if idx > defaultPairTokenIdx {
							defaultPool = poolID
							defaultPairToken = tkPair
							defaultPairTokenIdx = idx
							currentPoolAmount = pa
						}
						if (idx == defaultPairTokenIdx) && (pa > currentPoolAmount) {
							defaultPool = poolID
							defaultPairToken = tkPair
							defaultPairTokenIdx = idx
							currentPoolAmount = pa
						}
					}
				}

				if defaultPool == "" {
					if pa > 0 {
						defaultPool = poolID
						defaultPairToken = tkPair
						currentPoolAmount = pa
					}
				} else {
					if (pa > currentPoolAmount) && (defaultPairTokenIdx == -1) {
						defaultPool = poolID
						defaultPairToken = tkPair
						currentPoolAmount = pa
					}
				}
			}
		}
		data.DefaultPairToken = defaultPairToken
		data.DefaultPoolPair = defaultPool
		if data.TokenID == common.PRVCoinID.String() {
			data.PercentChange24h = fmt.Sprintf("%.2f", prvUsdtPair24h)
		} else {
			if data.DefaultPairToken != "" && data.TokenID != baseToken {
				data.PercentChange24h = fmt.Sprintf("%.2f", getToken24hPriceChange(data.TokenID, data.DefaultPairToken, data.DefaultPoolPair, baseToken, prvUsdtPair24h))
			}
		}

		if etki, ok := customTokenInfoMap[v.TokenID]; ok {
			if etki.Name != "" {
				data.Name = etki.Name
			}
			if etki.Symbol != "" {
				data.Symbol = etki.Symbol
			}
			if etki.Verified {
				data.Verified = etki.Verified
			}
		}
		if etki, ok := extraTokenInfoMap[v.TokenID]; ok {
			if etki.Name != "" {
				data.Name = etki.Name
			}
			data.Decimals = etki.Decimals
			if etki.Symbol != "" {
				data.Symbol = etki.Symbol
			}
			data.PSymbol = etki.PSymbol
			data.PDecimals = int(etki.PDecimals)
			data.ContractID = etki.ContractID
			data.Status = etki.Status
			data.Type = etki.Type
			data.CurrencyType = etki.CurrencyType
			data.Default = etki.Default
			if etki.Verified {
				data.Verified = etki.Verified
			}
			data.UserID = etki.UserID
			data.PercentChange1h = etki.PercentChange1h
			data.PercentChangePrv1h = etki.PercentChangePrv1h
			data.CurrentPrvPool = etki.CurrentPrvPool
			data.PricePrv = etki.PricePrv
			data.Volume24 = etki.Volume24
			data.ParentID = etki.ParentID
			data.OriginalSymbol = etki.OriginalSymbol
			data.LiquidityReward = etki.LiquidityReward
			data.Network = etki.Network
			err = json.UnmarshalFromString(etki.ListChildToken, &data.ListChildToken)
			if err != nil {
				panic(err)
			}
			if data.PriceUsd == 0 {
				data.PriceUsd = etki.PriceUsd
			}

		}

		if !v.IsNFT {
			datalist = append(datalist, data)
		}
	}

	for _, tkInfo := range extraTokenInfo {
		if _, ok := chainTkListMap[tkInfo.TokenID]; !ok {
			tkdata := TokenInfo{
				TokenID:      tkInfo.TokenID,
				Name:         tkInfo.Name,
				Symbol:       tkInfo.Symbol,
				PSymbol:      tkInfo.PSymbol,
				PDecimals:    int(tkInfo.PDecimals),
				Decimals:     tkInfo.Decimals,
				ContractID:   tkInfo.ContractID,
				Status:       tkInfo.Status,
				Type:         tkInfo.Type,
				CurrencyType: tkInfo.CurrencyType,
				Default:      tkInfo.Default,
				Verified:     tkInfo.Verified,
				UserID:       tkInfo.UserID,

				PriceUsd:           tkInfo.PriceUsd,
				PercentChange1h:    tkInfo.PercentChange1h,
				PercentChangePrv1h: tkInfo.PercentChangePrv1h,
				CurrentPrvPool:     tkInfo.CurrentPrvPool,
				PricePrv:           tkInfo.PricePrv,
				Volume24:           tkInfo.Volume24,
				ParentID:           tkInfo.ParentID,
				OriginalSymbol:     tkInfo.OriginalSymbol,
				LiquidityReward:    tkInfo.LiquidityReward,

				Network: tkInfo.Network,
			}
			err = json.UnmarshalFromString(tkInfo.ListChildToken, &tkdata.ListChildToken)
			if err != nil {
				panic(err)
			}
			datalist = append(datalist, tkdata)
		}
	}
	tokenListLock.Lock()
	defer tokenListLock.Unlock()
	marketTokenList = datalist
}
