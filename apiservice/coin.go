package apiservice

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/otaindexer"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/privacy"
	"github.com/incognitochain/incognito-chain/privacy/coin"
	"github.com/incognitochain/incognito-chain/wallet"
)

func APICheckKeyImages(c *gin.Context) {
	var req APICheckKeyImagesRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	if !req.Base58 {
		newList := []string{}
		for _, v := range req.Keyimages {
			s, err := base64.StdEncoding.DecodeString(v)
			if err != nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
				return
			}
			d := base58.EncodeCheck(s)
			newList = append(newList, d)
		}
		req.Keyimages = newList
	}

	result, err := database.DBCheckKeyimagesUsed(req.Keyimages, req.ShardID)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func APIGetRandomCommitments(c *gin.Context) {
	var req APIGetRandomCommitmentRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	commitmentIndices := []uint64{}
	myIndices := []uint64{}
	var publicKeys, commitments, assetTags []string

	if req.Version == 1 && len(req.Indexes) > 0 {
		result, err := database.DBGetCoinV1ByIndexes(req.Indexes, req.ShardID, req.TokenID)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		if len(req.Indexes) != len(result) {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("len(req.Indexes) != len(result)")))
			return
		}
		listUsableCommitments := make(map[common.Hash][]byte)
		listUsableCommitmentsIndices := make([]common.Hash, len(req.Indexes))
		mapIndexCommitmentsInUsableTx := make(map[string]*big.Int)
		usableInputCoins := []*coin.CoinV1{}
		for i, c := range result {
			coinV1 := new(coin.CoinV1)
			err := coinV1.SetBytes(c.Coin)
			if err != nil {
				panic(err)
			}
			usableInputCoins = append(usableInputCoins, coinV1)
			usableCommitment := coinV1.GetCommitment().ToBytesS()
			commitmentInHash := common.HashH(usableCommitment)
			listUsableCommitments[commitmentInHash] = usableCommitment
			listUsableCommitmentsIndices[i] = commitmentInHash
			commitmentInBase58Check := base58.Base58Check{}.Encode(usableCommitment, common.ZeroByte)
			mapIndexCommitmentsInUsableTx[commitmentInBase58Check] = new(big.Int).SetUint64(c.CoinIndex)
		}

		// loop to random commitmentIndexs
		cpRandNum := (len(listUsableCommitments) * privacy.CommitmentRingSize) - len(listUsableCommitments)
		//fmt.Printf("cpRandNum: %d\n", cpRandNum)
		lenCommitment := new(big.Int).SetInt64(database.DBGetCoinV1OfShardCount(req.ShardID, req.TokenID) - 1)
		if lenCommitment == nil {
			log.Println(errors.New("commitments is empty"))
			return
		}
		if lenCommitment.Uint64() == 1 && len(req.Indexes) == 1 {
			commitmentIndices = []uint64{0, 0, 0, 0, 0, 0, 0}
			temp := base58.EncodeCheck(usableInputCoins[0].GetCommitment().ToBytesS())
			commitments = []string{temp, temp, temp, temp, temp, temp, temp}
		} else {
			randIdxs := []uint64{}
			for {
				if len(randIdxs) == cpRandNum {
					break
				}
			random:
				index, _ := common.RandBigIntMaxRange(lenCommitment)
				for _, v := range mapIndexCommitmentsInUsableTx {
					if index.Uint64() == v.Uint64() {
						goto random
					}
				}
				randIdxs = append(randIdxs, index.Uint64())
			}

			coinList, err := database.DBGetCoinV1ByIndexes(randIdxs, req.ShardID, req.TokenID)
			if err != nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
				return
			}
			if len(randIdxs) != len(coinList) {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("len(randIdxs) != len(coinList)")))
				return
			}
			for _, c := range coinList {
				coinV1 := new(coin.CoinV1)
				err := coinV1.SetBytes(c.Coin)
				if err != nil {
					panic(err)
				}
				commitmentIndices = append(commitmentIndices, c.CoinIndex)
				if req.Base58 {
					commitments = append(commitments, base58.EncodeCheck(coinV1.GetCommitment().ToBytesS()))
				} else {
					commitments = append(commitments, base64.StdEncoding.EncodeToString(coinV1.GetCommitment().ToBytesS()))
				}
			}
		}
		// loop to insert usable commitments into commitmentIndexs for every group
		j := 0
		for _, commitmentInHash := range listUsableCommitmentsIndices {
			commitmentValue := base58.Base58Check{}.Encode(listUsableCommitments[commitmentInHash], common.ZeroByte)
			index := mapIndexCommitmentsInUsableTx[commitmentValue]
			randInt := rand.Intn(privacy.CommitmentRingSize)
			i := (j * privacy.CommitmentRingSize) + randInt
			commitmentIndices = append(commitmentIndices[:i], append([]uint64{index.Uint64()}, commitmentIndices[i:]...)...)
			if !req.Base58 {
				commitmentValue = base64.StdEncoding.EncodeToString(listUsableCommitments[commitmentInHash])
			}
			commitments = append(commitments[:i], append([]string{commitmentValue}, commitments[i:]...)...)
			myIndices = append(myIndices, uint64(i)) // create myCommitmentIndexs
			j++
		}
	}
	if req.Version == 2 && req.Limit > 0 {
		if req.TokenID != common.PRVCoinID.String() {
			req.TokenID = common.ConfidentialAssetID.String()
		}
	retry:
		count, err := database.DBGetCoinV2OfShardCount(req.ShardID, req.TokenID)
		if err != nil {
			fmt.Println(err)
			time.Sleep(100 * time.Millisecond)
			goto retry
		}
		lenOTA := new(big.Int).SetInt64(count - 1)
		var hasAssetTags bool = true
		for i := 0; i < req.Limit; i++ {
			idx, _ := common.RandBigIntMaxRange(lenOTA)
			log.Println("getRandomCommitmentsHandler", lenOTA, idx.Int64())
			coinIdx := uint64(idx.Int64())
			coinData, err := database.DBGetCoinsByIndex(coinIdx, req.ShardID, req.TokenID)
			if err != nil {
				i--
				continue
			}
			if coinData.CoinPubkey == shared.BurnCoinID {
				i--
				continue
			}
			coinV2 := new(coin.CoinV2)
			err = coinV2.SetBytes(coinData.Coin)
			if err != nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
				return
			}
			publicKey := coinV2.GetPublicKey()
			commitment := coinV2.GetCommitment()

			commitmentIndices = append(commitmentIndices, coinIdx)
			if req.Base58 {
				publicKeys = append(publicKeys, base58.EncodeCheck(publicKey.ToBytesS()))
				commitments = append(commitments, base58.EncodeCheck(commitment.ToBytesS()))
			} else {
				publicKeys = append(publicKeys, base64.StdEncoding.EncodeToString(publicKey.ToBytesS()))
				commitments = append(commitments, base64.StdEncoding.EncodeToString(commitment.ToBytesS()))
			}

			if hasAssetTags {
				assetTag := coinV2.GetAssetTag()
				if assetTag != nil {
					if req.Base58 {
						assetTags = append(assetTags, base58.EncodeCheck(assetTag.ToBytesS()))
					} else {
						assetTags = append(assetTags, base64.StdEncoding.EncodeToString(assetTag.ToBytesS()))
					}
				} else {
					hasAssetTags = false
				}
			}
		}
	}

	rs := struct {
		CommitmentIndices []uint64
		MyIndices         []uint64
		PublicKeys        []string
		Commitments       []string
		AssetTags         []string
	}{
		CommitmentIndices: commitmentIndices,
		MyIndices:         myIndices,
		PublicKeys:        publicKeys,
		Commitments:       commitments,
		AssetTags:         assetTags,
	}
	respond := APIRespond{
		Result: rs,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func APIGetTokenInfo(c *gin.Context) {
	var req APITokenInfoRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	var datalist []TokenInfo
	tokenList, err := database.DBGetTokenByTokenID(req.TokenIDs)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}
	extraTokenInfo, err := database.DBGetExtraTokenInfoByTokenID(req.TokenIDs)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}
	customTokenInfo, err := database.DBGetCustomTokenInfoByTokenID(req.TokenIDs)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}

	defaultPools, err := database.DBGetDefaultPool()
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	priorityTokens, err := database.DBGetTokenPriority()
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
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
		datalist = append(datalist, data)
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
	respond := APIRespond{
		Result: datalist,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func APIGetTokenList(c *gin.Context) {
	allToken := c.Query("all")
	includeNFT := c.Query("nft")
	var datalist []TokenInfo
	err := cacheGet(tokenInfoKey+string(allToken), &datalist)
	if err != nil {
		log.Println(err)
	}
	if len(datalist) == 0 {
		var tokenList []shared.TokenInfoData
		tokenList, err = database.DBGetAllTokenInfo()
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
			return
		}

		if includeNFT == "true" {
			nftList, err := database.DBGetNFTInfo()
			if err != nil {
				log.Println(err)
				c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
				return
			}
			tokenList = append(tokenList, nftList...)
		}
		extraTokenInfo, err := database.DBGetAllExtraTokenInfo()
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
			return
		}
		customTokenInfo, err := database.DBGetAllCustomTokenInfo()
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
			return
		}

		defaultPools, err := database.DBGetDefaultPool()
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		priorityTokens, err := database.DBGetTokenPriority()
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
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
			if includeNFT == "true" && v.IsNFT {
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
		err = cacheStoreCustom(tokenInfoKey+string(allToken), datalist, 20*time.Second)
		if err != nil {
			log.Println(err)
		}
	}
	respond := APIRespond{
		Result: datalist,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func APIGetCoinInfo(c *gin.Context) {
	prvV1, prvV2, tokenV1, tokenV2, err := database.DBGetCoinInfo()
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}
	result := struct {
		PRVV1   map[int]uint64
		PRVV2   map[int]uint64
		TokenV1 map[int]uint64
		TokenV2 map[int]uint64
	}{prvV1, prvV2, tokenV1, tokenV2}
	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func APIGetCoinsPending(c *gin.Context) {
	base58Fmt := c.Query("base58")
	result, err := database.DBGetPendingCoins()
	if err != nil {
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}
	if base58Fmt == "true" {
		resultB58 := []string{}
		for _, v := range result {
			vBytes, _, _ := base58.DecodeCheck(v)
			resultB58 = append(resultB58, base64.StdEncoding.EncodeToString(vBytes))
		}
		result = resultB58
	}
	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func APIGetPendingTxs(c *gin.Context) {
	base58Fmt := false

	if c.Query("base58") == "true" {
		base58Fmt = true
	}

	txdataStrs, err := database.DBGetPendingTxsData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}
	txDatas := []shared.TxData{}
	for _, v := range txdataStrs {
		txDatas = append(txDatas, shared.TxData{
			TxDetail:    v,
			BlockHeight: 0,
		})
	}
	txDetails, err := buildTxDetailRespond(txDatas, base58Fmt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}
	respond := APIRespond{
		Result: txDetails,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func APICheckTxPending(c *gin.Context) {
	txhash := c.Query("txhash")
	base58Fmt := false

	if c.Query("base58") == "true" {
		base58Fmt = true
	}

	result, err := database.DBGetPendingTxDetail(txhash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}

	txdata := shared.TxData{
		TxDetail:    result,
		BlockHeight: 0,
	}
	txDetails, err := buildTxDetailRespond([]shared.TxData{txdata}, base58Fmt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}
	respond := APIRespond{
		Result: txDetails,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func APIGetCoins(c *gin.Context) {
	version, _ := strconv.Atoi(c.Query("version"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	paymentkey := c.Query("paymentkey")
	viewkey := c.Query("viewkey")
	otakey := c.Query("otakey")
	tokenid := c.Query("tokenid")
	shardid, _ := strconv.Atoi(c.Query("shardid"))
	getNFT := false

	base58Format := false
	log.Println("tokenid", tokenid, common.PRVCoinID.String())
	if tokenid == "" {
		tokenid = common.PRVCoinID.String()
	}
	if c.Query("base58") == "true" {
		base58Format = true
	}
	if version != 1 && version != 2 {
		version = 1
	}
	if tokenid == "nft" {
		getNFT = true
	}
	var pubkey string
	highestIdx := uint64(0)
	if paymentkey == "" && viewkey == "" && otakey == "" && version == 2 {
		list, err := database.DBGetCoinsV2ByShardID(shardid, int64(limit), int64(offset))
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		var result []interface{}
		for _, cn := range list {
			if cn.CoinIndex > highestIdx {
				highestIdx = cn.CoinIndex
			}
			coinV2 := new(coin.CoinV2)
			err := coinV2.SetBytes(cn.Coin)
			if err != nil {
				c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
				return
			}
			idx := new(big.Int).SetUint64(cn.CoinIndex)
			var cV2 shared.OutCoinV2
			if base58Format {
				cV2 = shared.NewOutCoinV2(coinV2, true)
				cV2.Index = base58.Base58Check{}.Encode(idx.Bytes(), common.ZeroByte)
			} else {
				cV2 = shared.NewOutCoinV2(coinV2, false)
				cV2.Index = base64.StdEncoding.EncodeToString(idx.Bytes())
			}
			cV2.TxHash = cn.TxHash
			result = append(result, cV2)
		}
		rs := make(map[string]interface{})
		rs["HighestIndex"] = highestIdx
		rs["Outputs"] = result
		respond := APIRespond{
			Result: rs,
			Error:  nil,
		}
		c.JSON(http.StatusOK, respond)
	} else {
		if version == 2 {
			if otakey != "" {
				wl, err := wallet.Base58CheckDeserialize(otakey)
				if err != nil {
					c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
					return
				}
				if wl.KeySet.OTAKey.GetOTASecretKey() == nil {
					c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("invalid otakey")))
					return
				}
				pukeyBytes := wl.KeySet.OTAKey.GetPublicSpend().ToBytesS()
				pubkey = base58.EncodeCheck(pukeyBytes)
				shardID := common.GetShardIDFromLastByte(pukeyBytes[len(pukeyBytes)-1])
				if !getNFT {
					tokenidv2 := tokenid
					coinList, err := database.DBGetCoinsByOTAKey(int(shardID), tokenidv2, base58.EncodeCheck(wl.KeySet.OTAKey.GetOTASecretKey().ToBytesS()), int64(offset), int64(limit))
					if err != nil {
						c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
						return
					}

					var result []interface{}
					for _, cn := range coinList {
						if cn.CoinIndex > highestIdx {
							highestIdx = cn.CoinIndex
						}
						coinV2 := new(coin.CoinV2)
						err := coinV2.SetBytes(cn.Coin)
						if err != nil {
							c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
							return
						}
						idx := new(big.Int).SetUint64(cn.CoinIndex)
						var cV2 shared.OutCoinV2
						if base58Format {
							cV2 = shared.NewOutCoinV2(coinV2, true)
							cV2.Index = base58.Base58Check{}.Encode(idx.Bytes(), common.ZeroByte)
						} else {
							cV2 = shared.NewOutCoinV2(coinV2, false)
							cV2.Index = base64.StdEncoding.EncodeToString(idx.Bytes())
						}
						cV2.TxHash = cn.TxHash
						result = append(result, cV2)
					}
					rs := make(map[string]interface{})
					rs["HighestIndex"] = highestIdx
					rs["Outputs"] = map[string]interface{}{pubkey: result}
					respond := APIRespond{
						Result: rs,
						Error:  nil,
					}
					c.JSON(http.StatusOK, respond)
				} else {
					coinList, err := database.DBGetNFTByOTAKey(int(shardID), base58.EncodeCheck(wl.KeySet.OTAKey.GetOTASecretKey().ToBytesS()), int64(offset), int64(limit))
					if err != nil {
						c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
						return
					}
					result := make(map[string][]interface{})
					for token, list := range coinList {
						for _, cn := range list {
							if cn.CoinIndex > highestIdx {
								highestIdx = cn.CoinIndex
							}
							coinV2 := new(coin.CoinV2)
							err := coinV2.SetBytes(cn.Coin)
							if err != nil {
								c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
								return
							}
							idx := new(big.Int).SetUint64(cn.CoinIndex)
							var cV2 shared.OutCoinV2
							if base58Format {
								cV2 = shared.NewOutCoinV2(coinV2, true)
								cV2.Index = base58.Base58Check{}.Encode(idx.Bytes(), common.ZeroByte)
							} else {
								cV2 = shared.NewOutCoinV2(coinV2, false)
								cV2.Index = base64.StdEncoding.EncodeToString(idx.Bytes())
							}
							cV2.TxHash = cn.TxHash
							result[token] = append(result[token], cV2)
						}
					}
					rs := make(map[string]interface{})
					rs["HighestIndex"] = highestIdx
					rs["Outputs"] = result
					respond := APIRespond{
						Result: rs,
						Error:  nil,
					}
					c.JSON(http.StatusOK, respond)
				}

			} else {
				coinList, err := database.DBGetUnknownCoinsV21(shardid, tokenid, int64(offset), int64(limit))
				if err != nil {
					c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
					return
				}

				var result []interface{}
				for _, cn := range coinList {
					if cn.CoinIndex > highestIdx {
						highestIdx = cn.CoinIndex
					}
					coinV2 := new(coin.CoinV2)
					err := coinV2.SetBytes(cn.Coin)
					if err != nil {
						c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
						return
					}
					idx := new(big.Int).SetUint64(cn.CoinIndex)
					var cV2 shared.OutCoinV2
					if base58Format {
						cV2 = shared.NewOutCoinV2(coinV2, true)
						cV2.Index = base58.Base58Check{}.Encode(idx.Bytes(), common.ZeroByte)
					} else {
						cV2 = shared.NewOutCoinV2(coinV2, false)
						cV2.Index = base64.StdEncoding.EncodeToString(idx.Bytes())
					}
					cV2.TxHash = cn.TxHash
					result = append(result, cV2)
				}
				rs := make(map[string]interface{})
				rs["HighestIndex"] = highestIdx
				rs["Outputs"] = map[string]interface{}{pubkey: result}
				respond := APIRespond{
					Result: rs,
					Error:  nil,
				}
				c.JSON(http.StatusOK, respond)
			}
		}
		if version == 1 {
			var viewKeySet *incognitokey.KeySet
			if viewkey != "" {
				wl, err := wallet.Base58CheckDeserialize(viewkey)
				if err != nil {
					c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
					return
				}
				if wl.KeySet.ReadonlyKey.Rk == nil {
					c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("invalid viewkey")))
					return
				}
				pubkey = base58.EncodeCheck(wl.KeySet.ReadonlyKey.GetPublicSpend().ToBytesS())
				wl.KeySet.PaymentAddress.Pk = wl.KeySet.ReadonlyKey.Pk
				viewKeySet = &wl.KeySet
			} else {
				var err error
				pubkey, err = extractPubkeyFromKey(paymentkey, false)
				if err != nil {
					errStr := err.Error()
					respond := APIRespond{
						Result: nil,
						Error:  &errStr,
					}
					c.JSON(http.StatusOK, respond)
					return
				}
			}

			coinListV1, err := database.DBGetCoinV1ByPubkey(tokenid, pubkey, int64(offset), int64(limit))
			if err != nil {
				c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
				return
			}
			var wg sync.WaitGroup
			collectCh := make(chan shared.OutCoinV1, shared.MAX_CONCURRENT_COIN_DECRYPT)
			var result []interface{}
			for idx, cdata := range coinListV1 {
				if cdata.CoinIndex > highestIdx {
					highestIdx = cdata.CoinIndex
				}
				coinV1 := new(coin.CoinV1)
				err := coinV1.SetBytes(cdata.Coin)
				if err != nil {
					c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
					return
				}
				if viewKeySet != nil {
					wg.Add(1)
					go func(cn *coin.CoinV1, cData shared.CoinData) {
						plainCoin, err := cn.Decrypt(viewKeySet)
						if err != nil {
							c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
							return
						}
						// cV1 := shared.NewOutCoinV1(plainCoin)

						idx := new(big.Int).SetUint64(cData.CoinIndex)
						var cV1 shared.OutCoinV1
						if base58Format {
							cV1 = shared.NewOutCoinV1(plainCoin, true)
							cV1.Index = base58.Base58Check{}.Encode(idx.Bytes(), common.ZeroByte)
						} else {
							cV1 = shared.NewOutCoinV1(plainCoin, false)
							cV1.Index = base64.StdEncoding.EncodeToString(idx.Bytes())
						}
						cV1.TxHash = cData.TxHash
						collectCh <- cV1
						wg.Done()
					}(coinV1, cdata)
					if (idx+1)%shared.MAX_CONCURRENT_COIN_DECRYPT == 0 || idx+1 == len(coinListV1) {
						wg.Wait()
						close(collectCh)
						for coin := range collectCh {
							result = append(result, coin)
						}
						collectCh = make(chan shared.OutCoinV1, shared.MAX_CONCURRENT_COIN_DECRYPT)
					}
				} else {
					idx := new(big.Int).SetUint64(cdata.CoinIndex)
					var cV1 shared.OutCoinV1
					if base58Format {
						cV1 = shared.NewOutCoinV1(coinV1, true)
						cV1.Index = base58.Base58Check{}.Encode(idx.Bytes(), common.ZeroByte)
						cV1.CoinDetailsEncrypted = base58.Base58Check{}.Encode(coinV1.GetCoinDetailEncrypted(), common.ZeroByte)
					} else {
						cV1 = shared.NewOutCoinV1(coinV1, false)
						cV1.Index = base64.StdEncoding.EncodeToString(idx.Bytes())
						cV1.CoinDetailsEncrypted = base64.StdEncoding.EncodeToString(coinV1.GetCoinDetailEncrypted())
					}
					cV1.TxHash = cdata.TxHash
					result = append(result, cV1)
				}
			}

			rs := make(map[string]interface{})
			rs["HighestIndex"] = highestIdx
			rs["Outputs"] = map[string]interface{}{pubkey: result}
			respond := APIRespond{
				Result: rs,
				Error:  nil,
			}
			c.JSON(http.StatusOK, respond)
		}
	}

}

func APIGetKeyInfo(c *gin.Context) {
	version, _ := strconv.Atoi(c.Query("version"))
	if version != 1 && version != 2 {
		version = 1
	}
	key := c.Query("key")
	if key != "" {
		wl, err := wallet.Base58CheckDeserialize(key)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}

		pubkey := ""
		if wl.KeySet.OTAKey.GetPublicSpend() != nil {
			pubkey = base58.EncodeCheck(wl.KeySet.OTAKey.GetPublicSpend().ToBytesS())
		}
		if wl.KeySet.ReadonlyKey.GetPublicSpend() != nil && pubkey == "" {
			pubkey = base58.EncodeCheck(wl.KeySet.ReadonlyKey.GetPublicSpend().ToBytesS())
		}
		if wl.KeySet.PaymentAddress.GetPublicSpend() != nil && pubkey == "" {
			pubkey = base58.EncodeCheck(wl.KeySet.PaymentAddress.GetPublicSpend().ToBytesS())
		}
		if version == 1 {
			result, err := database.DBGetCoinV1PubkeyInfo(pubkey)
			if err != nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
				return
			}
			respond := APIRespond{
				Result: result,
				Error:  nil,
			}
			c.JSON(http.StatusOK, respond)
		} else {
			result, err := database.DBGetCoinV2PubkeyInfo(pubkey)
			if err != nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
				return
			}
			delete(result.CoinIndex, common.ConfidentialAssetID.String())
			respond := APIRespond{
				Result: result,
				Error:  nil,
			}
			c.JSON(http.StatusOK, respond)
		}
	} else {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("key cant be empty")))
		return
	}
}

func APIRescanOTA(c *gin.Context) {
	var req APISubmitOTAkeyRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	wl, err := wallet.Base58CheckDeserialize(req.OTAKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	if wl.KeySet.OTAKey.GetOTASecretKey() == nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("OTASecretKey is invalid")))
		return
	}
	otaKey := base58.EncodeCheck(wl.KeySet.OTAKey.GetOTASecretKey().ToBytesS())
	pubKey := base58.EncodeCheck(wl.KeySet.OTAKey.GetPublicSpend().ToBytesS())

	err = otaindexer.ReCheckOTAKey(otaKey, pubKey, true)
	respond := APIRespond{
		Result: "true",
	}
	if err != nil {
		errStr := err.Error()
		respond = APIRespond{
			Result: "false",
			Error:  &errStr,
		}
	}
	c.JSON(http.StatusOK, respond)
}

func APISubmitOTA(c *gin.Context) {
	var req APISubmitOTAkeyRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	wl, err := wallet.Base58CheckDeserialize(req.OTAKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	if wl.KeySet.OTAKey.GetOTASecretKey() == nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("OTASecretKey is invalid")))
		return
	}
	otaKey := base58.EncodeCheck(wl.KeySet.OTAKey.GetOTASecretKey().ToBytesS())
	pubKey := base58.EncodeCheck(wl.KeySet.OTAKey.GetPublicSpend().ToBytesS())

	newSubmitRequest := shared.NewSubmittedOTAKeyData(otaKey, pubKey, req.OTAKey, 0)
	resp := make(chan error)
	otaindexer.OTAAssignChn <- otaindexer.OTAAssignRequest{
		Key:     newSubmitRequest,
		FromNow: req.FromNow,
		Respond: resp,
	}
	err = <-resp
	errStr := ""
	if err != nil {
		if strings.Contains(err.Error(), "already exist") {
			respond := APIRespond{
				Result: "true",
			}
			c.JSON(http.StatusOK, respond)
			return
		}
		errStr = err.Error()
		respond := APIRespond{
			Result: "false",
			Error:  &errStr,
		}
		c.JSON(http.StatusOK, respond)
		return
	}
	respond := APIRespond{
		Result: "true",
	}
	c.JSON(http.StatusOK, respond)
}

func APISubmitOTAFullmode(c *gin.Context) {
	var req APISubmitOTAkeyRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	wl, err := wallet.Base58CheckDeserialize(req.OTAKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	if wl.KeySet.OTAKey.GetOTASecretKey() == nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("OTASecretKey is invalid")))
		return
	}
	otaKey := base58.EncodeCheck(wl.KeySet.OTAKey.GetOTASecretKey().ToBytesS())
	pubKey := base58.EncodeCheck(wl.KeySet.OTAKey.GetPublicSpend().ToBytesS())

	newSubmitRequest := shared.NewSubmittedOTAKeyData(otaKey, pubKey, req.OTAKey, 0)
	resp := make(chan error)
	otaindexer.OTAAssignChn <- otaindexer.OTAAssignRequest{
		Key:     newSubmitRequest,
		FromNow: req.FromNow,
		Respond: resp,
	}
	err = <-resp
	errStr := ""
	if err != nil {
		// if !mongo.IsDuplicateKeyError(err) {
		// 	c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		// 	return
		// }
		errStr = err.Error()
	}
	respond := APIRespond{
		Result: "true",
		Error:  &errStr,
	}
	c.JSON(http.StatusOK, respond)
}
