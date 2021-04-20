package main

import (
	"encoding/base64"
	"flag"

	devframework "github.com/0xkumi/incognito-dev-framework"
	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
)

func main() {
	argDBName := flag.String("dbname", "", "set db name")
	argDBAddr := flag.String("dbaddr", "", "reset mongo address")
	argFullNode := flag.String("fullnode", "", "set fullnode address")
	argToken := flag.String("token", common.PRVCoinID.String(), "set tokenID for recovery")
	argShardID := flag.Int("shardid", 0, "set shardID")
	flag.Parse()

	dbName := *argDBName
	mongoAddr := *argDBAddr
	fullNodeAddr := *argFullNode
	tokenID := *argToken
	err := database.ConnectDB(dbName, mongoAddr)
	if err != nil {
		panic(err)
	}

	node := devframework.NewRPCClient(fullNodeAddr)
	csvKeyImages, err := database.DBGetAllKeyImages(tokenID, *argShardID)
	if err != nil {
		panic(err)
	}
	fnKeyImageMap, err := node.API_ListSerialNumbers(tokenID, byte(*argShardID))
	if err != nil {
		panic(err)
	}
	missingList := make(map[string]struct{})
	if len(csvKeyImages) != len(csvKeyImages) {
		csvKeyImageMap := listToMap(csvKeyImages)
		for k, _ := range fnKeyImageMap {
			if _, ok := csvKeyImageMap[k]; !ok {
				missingList[k] = struct{}{}
			}
		}
	}
	if len(missingList) > 0 {
		listToInsert, err := mapToKeyImageData(missingList, tokenID, *argShardID)
		if err != nil {
			panic(err)
		}
		err = database.DBSaveUsedKeyimage(listToInsert)
		if err != nil {
			panic(err)
		}
	}
}

func listToMap(list []shared.KeyImageData) map[string]int {
	result := make(map[string]int)
	for idx, v := range list {
		kmBytes, err := base64.StdEncoding.DecodeString(v.KeyImage)
		if err != nil {
			panic(err)
		}
		kmB58 := base58.EncodeCheck(kmBytes)
		result[kmB58] = idx
	}
	return result
}

func mapToKeyImageData(list map[string]struct{}, tokenID string, shardID int) ([]shared.KeyImageData, error) {
	var result []shared.KeyImageData
	for k, _ := range list {
		kmBytes, _, err := base58.DecodeCheck(k)
		if err != nil {
			panic(err)
		}
		kmBase64 := base64.StdEncoding.EncodeToString(kmBytes)
		newKeyImage := shared.NewKeyImageData(tokenID, "", kmBase64, 0, shardID)
		result = append(result, *newKeyImage)
	}
	return result, nil
}
