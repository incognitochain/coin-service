package apiservice

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/incognitochain/coin-service/chainsynker"
	"github.com/incognitochain/incognito-chain/common"
	"log"
	"net/http"
	"strconv"
)

func APIGetOTACoinsByIndices(c *gin.Context) {
	shardID, _ := strconv.ParseInt(c.Query("shardID"), 10, 64)
	isToken, _ := strconv.ParseBool(c.Query("isToken"))
	idx, _ := strconv.ParseInt(c.Query("idx"), 10, 64)

	tokenID := common.PRVCoinID
	if isToken {
		tokenID = common.ConfidentialAssetID
	}
	res, err := chainsynker.GetOTACoinsByIndices(byte(shardID), tokenID, []uint64{uint64(idx)})
	if err != nil {
		log.Printf("[APIGetOTACoinsByIndices] error: %v\n", err)
		errStr := fmt.Sprintf("shard %v, index %v, isToken %v not retrievable", shardID, idx, isToken)
		respond := APIRespond{
			Result: nil,
			Error:  &errStr,
		}
		c.JSON(http.StatusOK, respond)
		return
	}
	respond := APIRespond{Result: res, Error: nil}
	c.JSON(http.StatusOK, respond)
}

func APIGetOTACoinLength(c *gin.Context) {
	res, err := chainsynker.GetOTACoinLength()
	if err != nil {
		log.Printf("[APIGetOTACoinLength] error: %v\n", err)
		errStr := err.Error()
		respond := APIRespond{
			Result: nil,
			Error:  &errStr,
		}
		c.JSON(http.StatusOK, respond)
		return
	}
	respond := APIRespond{Result: res, Error: nil}
	c.JSON(http.StatusOK, respond)
}
