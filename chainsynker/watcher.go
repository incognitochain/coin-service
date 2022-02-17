package chainsynker

import (
	"encoding/base64"
	"fmt"
	"log"
	"time"

	devframework "github.com/0xkumi/incognito-dev-framework"
	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/config"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"github.com/incognitochain/incognito-chain/privacy/operation"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
	"github.com/incognitochain/incognito-chain/transaction"
	"github.com/incognitochain/incognito-chain/wire"
)

func mempoolWatcher() {
	Localnode.OnReceive(devframework.MSG_TX, func(msg interface{}) {
		msgData := msg.(*wire.MessageTx)
		if msgData.Transaction.GetProof() != nil {
			var sn []string
			var pendingTxs []shared.CoinPendingData
			shardID := common.GetShardIDFromLastByte(msgData.Transaction.GetSenderAddrLastByte())
			txBytes, err := json.Marshal(msgData.Transaction)
			if err != nil {
				panic(err)
			}
			for _, c := range msgData.Transaction.GetProof().GetInputCoins() {
				sn = append(sn, base64.StdEncoding.EncodeToString(c.GetKeyImage().ToBytesS()))
			}
			if _, ok := msgData.Transaction.(transaction.TransactionToken); ok {
				txTokenData := msgData.Transaction.(transaction.TransactionToken).GetTxTokenData()
				for _, c := range txTokenData.TxNormal.GetProof().GetInputCoins() {
					sn = append(sn, base64.StdEncoding.EncodeToString(c.GetKeyImage().ToBytesS()))
				}
			}
			pendingTxs = append(pendingTxs, *shared.NewCoinPendingData(sn, int(shardID), msgData.Transaction.Hash().String(), string(txBytes), msgData.Transaction.GetLockTime()))
			err = database.DBSavePendingTx(pendingTxs)
			if err != nil {
				log.Println(err)
			}
		}
	})
	Localnode.OnReceive(devframework.MSG_TX_PRIVACYTOKEN, func(msg interface{}) {
		msgData := msg.(*wire.MessageTxPrivacyToken)
		if msgData.Transaction.GetProof() != nil {
			var sn []string
			var pendingTxs []shared.CoinPendingData
			shardID := common.GetShardIDFromLastByte(msgData.Transaction.GetSenderAddrLastByte())
			txBytes, err := json.Marshal(msgData.Transaction)
			if err != nil {
				panic(err)
			}
			for _, c := range msgData.Transaction.GetProof().GetInputCoins() {
				sn = append(sn, base64.StdEncoding.EncodeToString(c.GetKeyImage().ToBytesS()))
			}
			txTokenData := msgData.Transaction.(transaction.TransactionToken).GetTxTokenData()
			for _, c := range txTokenData.TxNormal.GetProof().GetInputCoins() {
				sn = append(sn, base64.StdEncoding.EncodeToString(c.GetKeyImage().ToBytesS()))
			}
			pendingTxs = append(pendingTxs, *shared.NewCoinPendingData(sn, int(shardID), msgData.Transaction.Hash().String(), string(txBytes), msgData.Transaction.GetLockTime()))
			err = database.DBSavePendingTx(pendingTxs)
			if err != nil {
				log.Println(err)
			}
		}
	})
	interval := time.NewTicker(10 * time.Second)
	for {
		<-interval.C
		txList, err := database.DBGetPendingTxs()
		if err != nil {
			log.Println(err)
			continue
		}
		txsToRemove := []string{}
		for shardID, txHashes := range txList {
			exist, err := database.DBCheckTxsExist(txHashes, shardID)
			if err != nil {
				log.Println(err)
				continue
			}
			for idx, v := range exist {
				if v {
					txsToRemove = append(txsToRemove, txHashes[idx])
				}
			}
		}
		err = database.DBDeletePendingTxs(txsToRemove)
		if err != nil {
			log.Println("hmm234", err)
			continue
		}
	}
}
func tokenListWatcher() {
	interval := time.NewTicker(5 * time.Second)
	activeShards := config.Param().ActiveShards
	for {
		<-interval.C
		shardStateDB := make(map[byte]*statedb.StateDB)
		for i := 0; i < activeShards; i++ {
			shardID := byte(i)
			if useFullnodeData {
				shardStateDB[shardID] = Localnode.GetBlockchain().GetBestStateTransactionStateDB(byte(shardID))
			} else {
				shardStateDB[shardID] = TransactionStateDB[shardID].Copy()
			}

		}

		tokenStates := make(map[common.Hash]*statedb.TokenState)
		nftToken := make(map[string]bool)
		for i := 0; i < activeShards; i++ {
			shardID := byte(i)
			m := statedb.ListPrivacyToken(shardStateDB[shardID])
			for newK, newV := range m {
				if v, ok := tokenStates[newK]; !ok {
					tokenStates[newK] = newV
					if database.DBIsTokenNFT(newK.String()) {
						nftToken[newK.String()] = true
					}
				} else {
					if v.PropertyName() == "" && newV.PropertyName() != "" {
						v.SetPropertyName(newV.PropertyName())
					}
					if v.PropertySymbol() == "" && newV.PropertySymbol() != "" {
						v.SetPropertySymbol(newV.PropertySymbol())
					}
					if v.Amount() == 0 && newV.Amount() > 0 {
						v.SetAmount(newV.Amount())
					}
					if database.DBIsTokenNFT(newK.String()) {
						nftToken[newK.String()] = true
					} else {
						tokenStates[newK] = v
					}
				}
			}
		}

		tokenList := jsonresult.ListCustomToken{ListCustomToken: []jsonresult.CustomToken{}}
		for _, tokenState := range tokenStates {
			item := jsonresult.NewPrivacyToken(tokenState)
			tokenList.ListCustomToken = append(tokenList.ListCustomToken, *item)
		}

		_, allBridgeTokens, err := Localnode.GetBlockchain().GetAllBridgeTokens()
		if err != nil {
			log.Println(err)
			continue
		}

		for _, bridgeToken := range allBridgeTokens {
			if _, ok := tokenStates[*bridgeToken.TokenID]; ok {
				continue
			}
			item := jsonresult.CustomToken{
				ID:            bridgeToken.TokenID.String(),
				IsPrivacy:     true,
				IsBridgeToken: true,
			}
			if item.Name == "" {
				for i := 0; i < activeShards; i++ {
					shardID := byte(i)
					tokenState, has, err := statedb.GetPrivacyTokenState(shardStateDB[shardID], *bridgeToken.TokenID)
					if err != nil {
						log.Println(err)
					}
					if has {
						item.Name = tokenState.PropertyName()
						item.Symbol = tokenState.PropertySymbol()
						break
					}
				}
			}
			tokenList.ListCustomToken = append(tokenList.ListCustomToken, item)
		}
		externalIDs := make(map[string]string)
		for index, _ := range tokenList.ListCustomToken {
			tokenList.ListCustomToken[index].ListTxs = []string{}
			tokenList.ListCustomToken[index].Image = common.Render([]byte(tokenList.ListCustomToken[index].ID))
			for _, bridgeToken := range allBridgeTokens {
				if tokenList.ListCustomToken[index].ID == bridgeToken.TokenID.String() {
					externalIDs[bridgeToken.TokenID.String()] = fmt.Sprintf("%v", bridgeToken.ExternalTokenID)
					tokenList.ListCustomToken[index].Amount = bridgeToken.Amount
					tokenList.ListCustomToken[index].IsBridgeToken = true
					break
				}
			}
		}

		var tokenInfoList []shared.TokenInfoData
		for _, token := range tokenList.ListCustomToken {
			externalID := ""
			if id, ok := externalIDs[token.ID]; ok {
				externalID = id
			}
			tokenInfo := shared.NewTokenInfoData(token.ID, token.Name, token.Symbol, token.Image, token.IsPrivacy, token.IsBridgeToken, token.Amount, nftToken[token.ID], externalID)
			tokenInfoList = append(tokenInfoList, *tokenInfo)
		}
		err = database.DBSaveTokenInfo(tokenInfoList)
		if err != nil {
			panic(err)
		}
		lastTokenIDLock.Lock()

		if len(lastTokenIDMap) < len(tokenInfoList) {
			for _, tokenInfo := range tokenInfoList {
				tokenID, err := new(common.Hash).NewHashFromStr(tokenInfo.TokenID)
				if err != nil {
					panic(err)
				}
				recomputedAssetTag := operation.HashToPoint(tokenID[:])
				lastTokenIDMap[recomputedAssetTag.String()] = tokenInfo.TokenID
			}
		}

		lastTokenIDLock.Unlock()
	}
}
