package apiservice

import (
	"encoding/base64"
	"errors"
	"log"
	"math/big"
	"math/rand"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/privacy"
	"github.com/incognitochain/incognito-chain/privacy/coin"
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
		lenOTA := new(big.Int).SetInt64(database.DBGetCoinV2OfShardCount(req.ShardID, req.TokenID) - 1)
		var hasAssetTags bool = true
		for i := 0; i < req.Limit; i++ {
			idx, _ := common.RandBigIntMaxRange(lenOTA)
			log.Println("getRandomCommitmentsHandler", lenOTA, idx.Int64())
			coinData, err := database.DBGetCoinsByIndex(int(idx.Int64()), req.ShardID, req.TokenID)
			if err != nil {
				i--
				continue
			}
			if coinData.CoinPubkey == "1y4gnYS1Ns2K7BjQTjgfZ5nTR8JZMkMJ3CTGMj2Pk7CQkSTFgA" {
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

			commitmentIndices = append(commitmentIndices, idx.Uint64())
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

func APIGetCoinInfo(c *gin.Context) {
	prvV1, prvV2, tokenV1, tokenV2, err := database.DBGetCoinInfo()
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}
	result := struct {
		TotalPRVV1   int64
		TotalPRVV2   int64
		TotalTokenV1 int64
		TotalTokenV2 int64
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
