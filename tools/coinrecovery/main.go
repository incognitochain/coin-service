package main

import (
	"encoding/hex"
	"flag"
	"math/big"

	devframework "github.com/0xkumi/incognito-dev-framework"
	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/privacy/coin"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
	"github.com/incognitochain/incognito-chain/wallet"
)

func main() {
	argDBName := flag.String("dbname", "", "set db name")
	argDBAddr := flag.String("dbaddr", "", "reset mongo address")
	argFullNode := flag.String("fullnode", "", "set fullnode address")
	argCoinV1 := flag.Bool("v1", true, "set coin version")
	argKey := flag.String("key", "", "set otakey/pubkey for recovery")
	argToken := flag.String("token", common.PRVCoinID.String(), "set tokenID for recovery")
	argNumberOfShards := flag.Int("shards", 8, "set number of shard")
	flag.Parse()

	dbName := *argDBName
	mongoAddr := *argDBAddr
	isCoinV1 := *argCoinV1
	userKey := *argKey
	fullNodeAddr := *argFullNode
	tokenID := *argToken
	common.MaxShardNumber = *argNumberOfShards
	err := database.ConnectDB(dbName, mongoAddr)
	if err != nil {
		panic(err)
	}

	node := devframework.NewRPCClient(fullNodeAddr)

	wl, err := wallet.Base58CheckDeserialize(userKey)
	if err != nil {
		panic(err)
	}
	if isCoinV1 {
		pubKey := hex.EncodeToString(wl.KeySet.PaymentAddress.Pk)
		shardID := common.GetShardIDFromLastByte(wl.KeySet.PaymentAddress.Pk[len(wl.KeySet.PaymentAddress.Pk)-1])
		fnCs, err := node.API_ListOutputCoins(userKey, "", "", tokenID, 999999999)
		if err != nil {
			panic(err)
		}
		idxList := make(map[string]uint64)
		fnCoins := []coin.CoinV1{}
		for _, coinList := range fnCs.Outputs {
			for _, cnJson := range coinList {
				cn, _, err := jsonresult.NewCoinFromJsonOutCoin(cnJson)
				if err != nil {
					panic(err)
				}
				cv1 := cn.(coin.CoinV1)
				fnCoins = append(fnCoins, cv1)
				idxBytes, _, err := base58.DecodeCheck(cnJson.Index)
				if err != nil {
					panic(err)
				}
				idxBigInt := new(big.Int).SetBytes(idxBytes)
				idxList[cn.GetCommitment().String()] = idxBigInt.Uint64()
			}
		}
		csCs, err := database.DBGetCoinV1ByPubkey(tokenID, pubKey, 0, 999999999)
		if err != nil {
			panic(err)
		}
		csCoins, err := convertToCoinV1(csCs)
		if err != nil {
			panic(err)
		}
		diffCoins := differenceV1(fnCoins, csCoins)
		if len(diffCoins) > 0 {
			cdata, err := convertToDBCoinV1Data(idxList, diffCoins, int(shardID), tokenID)
			if err != nil {
				panic(err)
			}
			err = database.DBSaveCoins(cdata)
			if err != nil {
				panic(err)
			}
		}
	} else {

	}
}

func convertToDBCoinV1Data(idxList map[string]uint64, list []coin.CoinV1, shardID int, tokenID string) ([]shared.CoinData, error) {
	var result []shared.CoinData
	for _, coin := range list {
		outCoin := shared.NewCoinData(0, idxList[coin.GetCommitment().String()], coin.Bytes(), tokenID, coin.GetPublicKey().String(), "", "", shardID, 1)
		result = append(result, *outCoin)
		return result, nil
	}
}

func convertToCoinV1(list []shared.CoinData) ([]coin.CoinV1, error) {
	var result []coin.CoinV1
	for _, cn := range list {
		cv1 := new(coin.CoinV1).Init()
		cv1.SetBytes(cn.Coin)
		result = append(result, *cv1)
	}
	return result, nil
}

func differenceV1(a, b []coin.CoinV1) []coin.CoinV1 {
	mb := make(map[string]coin.CoinV1, len(b))
	for _, x := range b {
		mb[x.GetCommitment().String()] = x
	}
	var diff []coin.CoinV1
	for _, x := range a {
		if _, found := mb[x.GetCommitment().String()]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}
