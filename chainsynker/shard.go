package chainsynker

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/blockchain/types"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"github.com/incognitochain/incognito-chain/metadata"
	metadataCommon "github.com/incognitochain/incognito-chain/metadata/common"
	"github.com/incognitochain/incognito-chain/transaction"
	"github.com/incognitochain/incognito-chain/wallet"
)

func OnNewShardBlock(bc *blockchain.BlockChain, h common.Hash, height uint64) {
	var blk types.ShardBlock
	blkBytes, err := Localnode.GetUserDatabase().Get(h.Bytes(), nil)
	if err != nil {
		fmt.Println("height", height, h.String())
		panic(err)
	}
	if err := json.Unmarshal(blkBytes, &blk); err != nil {
		panic(err)
	}
	blockHash := blk.Hash().String()
	blockHeight := blk.GetHeight()
	shardID := blk.GetShardID()
	log.Printf("start processing coin for block %v shard %v\n", blk.GetHeight(), shardID)
	startTime := time.Now()
	if len(blk.Body.Transactions) > 0 {
		err = bc.CreateAndSaveTxViewPointFromBlock(&blk, TransactionStateDB[byte(shardID)])
		if err != nil {
			panic(err)
		}
	}
	// Store Incomming Cross Shard
	if len(blk.Body.CrossTransactions) > 0 {
		if err := bc.CreateAndSaveCrossTransactionViewPointFromBlock(&blk, TransactionStateDB[byte(shardID)]); err != nil {
			panic(err)
		}
	}
	transactionRootHash, err := TransactionStateDB[byte(shardID)].Commit(true)
	if err != nil {
		panic(err)
	}
	err = TransactionStateDB[byte(shardID)].Database().TrieDB().Commit(transactionRootHash, false)
	if err != nil {
		panic(err)
	}
	bc.GetBestStateShard(byte(blk.GetShardID())).TransactionStateDBRootHash = transactionRootHash

	TransactionStateDB[byte(shardID)].ClearObjects()

	batchData := bc.GetShardChainDatabase(blk.Header.ShardID).NewBatch()
	err = bc.BackupShardViews(batchData, blk.Header.ShardID)
	if err != nil {
		panic("Backup shard view error")
	}
	if err := batchData.Write(); err != nil {
		panic(err)
	}

	crossShardCoinMap := make(map[string]string)
	for _, txlist := range blk.Body.CrossTransactions {
		for _, tx := range txlist {
			var crsblk types.ShardBlock
		retryGetBlock:
			blkBytes, err := Localnode.GetUserDatabase().Get(tx.BlockHash.Bytes(), nil)
			if err != nil {
				log.Println(err)
				time.Sleep(5 * time.Second)
				goto retryGetBlock
			}
			if err := json.Unmarshal(blkBytes, &crsblk); err != nil {
				panic(err)
			}
			err = getCrossShardData(crossShardCoinMap, crsblk.Body.Transactions, byte(shardID))
			if err != nil {
				panic(err)
			}
		}
	}

	//store output-coin and keyimage on db
	keyImageList := []shared.KeyImageData{}
	outCoinList := []shared.CoinData{}
	beaconHeight := blk.Header.BeaconHeight
	coinV1PubkeyInfo := make(map[string]map[string]shared.CoinInfo)

	txDataList := []shared.TxData{}

	for _, txs := range blk.Body.CrossTransactions {
		for _, tx := range txs {
			for _, prvout := range tx.OutputCoin {
				publicKeyBytes := prvout.GetPublicKey().ToBytesS()
				publicKeyStr := base58.EncodeCheck(publicKeyBytes)
				publicKeyShardID := common.GetShardIDFromLastByte(publicKeyBytes[len(publicKeyBytes)-1])
				if publicKeyShardID == byte(shardID) {
					coinIdx := uint64(0)
					if prvout.GetVersion() == 2 {
						idxBig, err := statedb.GetOTACoinIndex(TransactionStateDB[byte(blk.GetShardID())], common.PRVCoinID, publicKeyBytes)
						if err != nil {
							log.Println("len(outs))", len(tx.OutputCoin), base58.Base58Check{}.Encode(publicKeyBytes, 0))
							panic(err)
						}
						coinIdx = idxBig.Uint64()
					} else {
						idxBig, err := statedb.GetCommitmentIndex(TransactionStateDB[byte(blk.GetShardID())], common.PRVCoinID, prvout.GetCommitment().ToBytesS(), byte(blk.GetShardID()))
						if err != nil {
							panic(err)
						}
						coinIdx = idxBig.Uint64()
						if _, ok := coinV1PubkeyInfo[publicKeyStr]; !ok {
							coinV1PubkeyInfo[publicKeyStr] = make(map[string]shared.CoinInfo)
						}
						if _, ok := coinV1PubkeyInfo[publicKeyStr][common.PRVCoinID.String()]; !ok {
							coinV1PubkeyInfo[publicKeyStr][common.PRVCoinID.String()] = shared.CoinInfo{
								Start: coinIdx,
								Total: 1,
								End:   coinIdx,
							}
						} else {
							newCoinInfo := coinV1PubkeyInfo[publicKeyStr][common.PRVCoinID.String()]
							newCoinInfo.Total = newCoinInfo.Total + 1
							if coinIdx > newCoinInfo.End {
								newCoinInfo.End = coinIdx
							}
							if coinIdx < newCoinInfo.Start {
								newCoinInfo.Start = coinIdx
							}
							coinV1PubkeyInfo[publicKeyStr][common.PRVCoinID.String()] = newCoinInfo
						}
					}
					outCoin := shared.NewCoinData(beaconHeight, coinIdx, prvout.Bytes(), common.PRVCoinID.String(), publicKeyStr, "", crossShardCoinMap[prvout.GetCommitment().String()], shardID, int(prvout.GetVersion()))
					outCoinList = append(outCoinList, *outCoin)
				}
			}
			for _, tkouts := range tx.TokenPrivacyData {
				for _, tkout := range tkouts.OutputCoin {
					publicKeyBytes := tkout.GetPublicKey().ToBytesS()
					publicKeyStr := base58.EncodeCheck(publicKeyBytes)
					publicKeyShardID := common.GetShardIDFromLastByte(publicKeyBytes[len(publicKeyBytes)-1])
					if publicKeyShardID == byte(shardID) {
						coinIdx := uint64(0)
						tokenStr := tkouts.PropertyID.String()
						if tkout.GetVersion() == 2 {
							idxBig, err := statedb.GetOTACoinIndex(TransactionStateDB[byte(blk.GetShardID())], tkouts.PropertyID, publicKeyBytes)
							if err != nil {
								panic(err)
							}
							coinIdx = idxBig.Uint64()
							tokenStr = common.ConfidentialAssetID.String()
						} else {
							idxBig, err := statedb.GetCommitmentIndex(TransactionStateDB[byte(blk.GetShardID())], tkouts.PropertyID, tkout.GetCommitment().ToBytesS(), byte(blk.GetShardID()))
							if err != nil {
								panic(err)
							}
							coinIdx = idxBig.Uint64()
							if _, ok := coinV1PubkeyInfo[publicKeyStr]; !ok {
								coinV1PubkeyInfo[publicKeyStr] = make(map[string]shared.CoinInfo)
							}
							if _, ok := coinV1PubkeyInfo[publicKeyStr][tokenStr]; !ok {
								coinV1PubkeyInfo[publicKeyStr][tokenStr] = shared.CoinInfo{
									Start: coinIdx,
									Total: 1,
									End:   coinIdx,
								}
							} else {
								newCoinInfo := coinV1PubkeyInfo[publicKeyStr][tokenStr]
								newCoinInfo.Total = newCoinInfo.Total + 1
								if coinIdx > newCoinInfo.End {
									newCoinInfo.End = coinIdx
								}
								if coinIdx < newCoinInfo.Start {
									newCoinInfo.Start = coinIdx
								}
								coinV1PubkeyInfo[publicKeyStr][tokenStr] = newCoinInfo
							}
						}
						outCoin := shared.NewCoinData(beaconHeight, coinIdx, tkout.Bytes(), tokenStr, publicKeyStr, "", crossShardCoinMap[tkout.GetCommitment().String()], shardID, int(tkout.GetVersion()))
						outCoinList = append(outCoinList, *outCoin)
					}
				}
			}
		}
	}

	for _, tx := range blk.Body.Transactions {
		isCoinV2Output := false
		txHash := tx.Hash().String()
		tokenID := tx.GetTokenID().String()
		realTokenID := ""
		pubkey := ""
		isNFT := false
		txKeyImages := []string{}
		if tx.GetType() == common.TxNormalType || tx.GetType() == common.TxConversionType || tx.GetType() == common.TxRewardType || tx.GetType() == common.TxReturnStakingType {
			if tx.GetProof() == nil {
				continue
			}
			ins := tx.GetProof().GetInputCoins()
			outs := tx.GetProof().GetOutputCoins()

			for _, coin := range ins {
				kmString := base58.EncodeCheck(coin.GetKeyImage().ToBytesS())
				txKeyImages = append(txKeyImages, kmString)
				kmData := shared.NewKeyImageData(tokenID, txHash, kmString, beaconHeight, shardID)
				keyImageList = append(keyImageList, *kmData)
			}
			for _, coin := range outs {
				publicKeyBytes := coin.GetPublicKey().ToBytesS()
				publicKeyStr := base58.EncodeCheck(publicKeyBytes)
				publicKeyShardID := common.GetShardIDFromLastByte(publicKeyBytes[len(publicKeyBytes)-1])
				if publicKeyShardID == byte(shardID) {
					coinIdx := uint64(0)
					if coin.GetVersion() == 2 {
						isCoinV2Output = true
						idxBig, err := statedb.GetOTACoinIndex(TransactionStateDB[byte(blk.GetShardID())], common.PRVCoinID, publicKeyBytes)
						if err != nil {
							log.Println("len(outs))", len(outs), base58.Base58Check{}.Encode(publicKeyBytes, 0))
							panic(err)
						}
						coinIdx = idxBig.Uint64()
					} else {
						idxBig, err := statedb.GetCommitmentIndex(TransactionStateDB[byte(blk.GetShardID())], common.PRVCoinID, coin.GetCommitment().ToBytesS(), byte(blk.GetShardID()))
						if err != nil {
							panic(err)
						}
						coinIdx = idxBig.Uint64()
						if _, ok := coinV1PubkeyInfo[publicKeyStr]; !ok {
							coinV1PubkeyInfo[publicKeyStr] = make(map[string]shared.CoinInfo)
						}
						if _, ok := coinV1PubkeyInfo[publicKeyStr][common.PRVCoinID.String()]; !ok {
							coinV1PubkeyInfo[publicKeyStr][common.PRVCoinID.String()] = shared.CoinInfo{
								Start: coinIdx,
								Total: 1,
								End:   coinIdx,
							}
						} else {
							newCoinInfo := coinV1PubkeyInfo[publicKeyStr][common.PRVCoinID.String()]
							newCoinInfo.Total = newCoinInfo.Total + 1
							if coinIdx > newCoinInfo.End {
								newCoinInfo.End = coinIdx
							}
							if coinIdx < newCoinInfo.Start {
								newCoinInfo.Start = coinIdx
							}
							coinV1PubkeyInfo[publicKeyStr][common.PRVCoinID.String()] = newCoinInfo
						}
					}
					outCoin := shared.NewCoinData(beaconHeight, coinIdx, coin.Bytes(), tokenID, publicKeyStr, "", txHash, shardID, int(coin.GetVersion()))
					outCoinList = append(outCoinList, *outCoin)
				}
			}
		}
		if tx.GetType() == common.TxCustomTokenPrivacyType || tx.GetType() == common.TxTokenConversionType {
			txToken := tx.(transaction.TransactionToken)
			txTokenData := txToken.GetTxTokenData()

			if tx.GetMetadataType() == metadataCommon.Pdexv3MintNftResponseMeta || tx.GetMetadataType() == metadataCommon.Pdexv3UserMintNftResponseMeta {
				isNFT = true
			}

			if txTokenData.TxNormal.GetProof() != nil {
				tokenIns := txTokenData.TxNormal.GetProof().GetInputCoins()
				tokenOuts := txTokenData.TxNormal.GetProof().GetOutputCoins()

				for _, coin := range tokenIns {
					kmString := base58.EncodeCheck(coin.GetKeyImage().ToBytesS())
					txKeyImages = append(txKeyImages, kmString)
					kmData := shared.NewKeyImageData(common.ConfidentialAssetID.String(), txHash, kmString, beaconHeight, shardID)
					keyImageList = append(keyImageList, *kmData)
				}
				for _, coin := range tokenOuts {
					publicKeyBytes := coin.GetPublicKey().ToBytesS()
					publicKeyStr := base58.EncodeCheck(publicKeyBytes)
					publicKeyShardID := common.GetShardIDFromLastByte(publicKeyBytes[len(publicKeyBytes)-1])
					if publicKeyShardID == byte(shardID) {
						coinIdx := uint64(0)
						tokenStr := txToken.GetTokenID().String()
						if coin.GetVersion() == 2 {
							isCoinV2Output = true
							idxBig, err := statedb.GetOTACoinIndex(TransactionStateDB[byte(blk.GetShardID())], *tx.GetTokenID(), publicKeyBytes)
							if err != nil {
								panic(err)
							}
							coinIdx = idxBig.Uint64()
							tokenStr = common.ConfidentialAssetID.String()
						} else {
							idxBig, err := statedb.GetCommitmentIndex(TransactionStateDB[byte(blk.GetShardID())], *txToken.GetTokenID(), coin.GetCommitment().ToBytesS(), byte(blk.GetShardID()))
							if err != nil {
								panic(err)
							}
							coinIdx = idxBig.Uint64()
							if _, ok := coinV1PubkeyInfo[publicKeyStr]; !ok {
								coinV1PubkeyInfo[publicKeyStr] = make(map[string]shared.CoinInfo)
							}
							if _, ok := coinV1PubkeyInfo[publicKeyStr][tokenStr]; !ok {
								coinV1PubkeyInfo[publicKeyStr][tokenStr] = shared.CoinInfo{
									Start: coinIdx,
									Total: 1,
									End:   coinIdx,
								}
							} else {
								newCoinInfo := coinV1PubkeyInfo[publicKeyStr][tokenStr]
								newCoinInfo.Total = newCoinInfo.Total + 1
								if coinIdx > newCoinInfo.End {
									newCoinInfo.End = coinIdx
								}
								if coinIdx < newCoinInfo.Start {
									newCoinInfo.Start = coinIdx
								}
								coinV1PubkeyInfo[publicKeyStr][tokenStr] = newCoinInfo
							}
						}
						outCoin := shared.NewCoinData(beaconHeight, coinIdx, coin.Bytes(), tokenStr, publicKeyStr, "", txHash, shardID, int(coin.GetVersion()))
						outCoin.IsNFT = isNFT
						if isNFT {
							outCoin.RealTokenID = tokenID
						}
						outCoinList = append(outCoinList, *outCoin)
					}
				}
			}
			if tx.GetTxFee() > 0 {
				ins := tx.GetProof().GetInputCoins()
				outs := tx.GetProof().GetOutputCoins()
				for _, coin := range ins {
					kmString := base58.EncodeCheck(coin.GetKeyImage().ToBytesS())
					txKeyImages = append(txKeyImages, kmString)
					kmData := shared.NewKeyImageData(common.PRVCoinID.String(), txHash, kmString, beaconHeight, shardID)
					keyImageList = append(keyImageList, *kmData)
				}
				for _, coin := range outs {
					publicKeyBytes := coin.GetPublicKey().ToBytesS()
					publicKeyStr := base58.EncodeCheck(publicKeyBytes)
					publicKeyShardID := common.GetShardIDFromLastByte(publicKeyBytes[len(publicKeyBytes)-1])
					if publicKeyShardID == byte(shardID) {
						coinIdx := uint64(0)
						if coin.GetVersion() == 2 {
							idxBig, err := statedb.GetOTACoinIndex(TransactionStateDB[byte(blk.GetShardID())], common.PRVCoinID, publicKeyBytes)
							if err != nil {
								panic(err)
							}
							coinIdx = idxBig.Uint64()
						} else {
							idxBig, err := statedb.GetCommitmentIndex(TransactionStateDB[byte(blk.GetShardID())], common.PRVCoinID, coin.GetCommitment().ToBytesS(), byte(blk.GetShardID()))
							if err != nil {
								panic(err)
							}
							coinIdx = idxBig.Uint64()
							if _, ok := coinV1PubkeyInfo[publicKeyStr]; !ok {
								coinV1PubkeyInfo[publicKeyStr] = make(map[string]shared.CoinInfo)
							}
							if _, ok := coinV1PubkeyInfo[publicKeyStr][common.PRVCoinID.String()]; !ok {
								coinV1PubkeyInfo[publicKeyStr][common.PRVCoinID.String()] = shared.CoinInfo{
									Start: coinIdx,
									Total: 1,
									End:   coinIdx,
								}
							} else {
								newCoinInfo := coinV1PubkeyInfo[publicKeyStr][common.PRVCoinID.String()]
								newCoinInfo.Total = newCoinInfo.Total + 1
								if coinIdx > newCoinInfo.End {
									newCoinInfo.End = coinIdx
								}
								if coinIdx < newCoinInfo.Start {
									newCoinInfo.Start = coinIdx
								}
								coinV1PubkeyInfo[publicKeyStr][common.PRVCoinID.String()] = newCoinInfo
							}
						}
						outCoin := shared.NewCoinData(beaconHeight, coinIdx, coin.Bytes(), common.PRVCoinID.String(), publicKeyStr, "", txHash, shardID, int(coin.GetVersion()))
						outCoinList = append(outCoinList, *outCoin)
					}
				}
			}
		}

		pubkeyReceivers := []string{}
		if !isCoinV2Output {
			receiverList, _ := tx.GetReceivers()
			for _, v := range receiverList {
				pubkeyReceivers = append(pubkeyReceivers, base58.EncodeCheck(v))
			}
			if tx.GetType() == common.TxCustomTokenPrivacyType || tx.GetType() == common.TxTokenConversionType {
				txToken := tx.(transaction.TransactionToken)
				receiverList, _ := txToken.GetTxTokenData().TxNormal.GetReceivers()
				for _, v := range receiverList {
					pubkeyReceivers = append(pubkeyReceivers, base58.EncodeCheck(v))
				}
			}
		}

		txBytes, err := json.Marshal(tx)
		if err != nil {
			panic(err)
		}

		metaDataType := tx.GetMetadataType()
		switch metaDataType {
		case metadata.PDECrossPoolTradeRequestMeta, metadata.PDETradeRequestMeta:
			switch metaDataType {
			case metadata.PDECrossPoolTradeRequestMeta:
				meta := tx.GetMetadata().(*metadata.PDETradeRequest)
				payment := meta.TraderAddressStr
				wl, err := wallet.Base58CheckDeserialize(payment)
				if err != nil {
					panic(err)
				}
				pubkey = base58.EncodeCheck(wl.KeySet.PaymentAddress.Pk)
			case metadata.PDETradeRequestMeta:
				meta := tx.GetMetadata().(*metadata.PDECrossPoolTradeRequest)
				payment := meta.TraderAddressStr
				wl, err := wallet.Base58CheckDeserialize(payment)
				if err != nil {
					panic(err)
				}
				pubkey = base58.EncodeCheck(wl.KeySet.PaymentAddress.Pk)
			}
		case metadata.BurningRequestMeta, metadata.BurningRequestMetaV2, metadata.BurningForDepositToSCRequestMeta, metadata.BurningForDepositToSCRequestMetaV2, metadata.BurningPBSCRequestMeta:
			burningReqAction := tx.GetMetadata().(*metadata.BurningRequest)
			realTokenID = burningReqAction.TokenID.String()
			pubkey = base58.EncodeCheck(burningReqAction.BurnerAddress.GetPublicSpend().ToBytesS())
		case metadata.ContractingRequestMeta:
			contractingReqAction := tx.GetMetadata().(*metadata.ContractingRequest)
			realTokenID = contractingReqAction.TokenID.String()
			pubkey = base58.EncodeCheck(contractingReqAction.BurnerAddress.GetPublicSpend().ToBytesS())
		}

		mtd := ""
		if tx.GetMetadata() != nil {
			mtdBytes, err := json.Marshal(tx.GetMetadata())
			if err != nil {
				panic(err)
			}
			mtd = string(mtdBytes)
		}

		txData := shared.NewTxData(tx.GetLockTime(), shardID, int(tx.GetVersion()), blockHeight, blockHash, tokenID, txHash, tx.GetType(), string(txBytes), strconv.Itoa(metaDataType), mtd, txKeyImages, pubkeyReceivers, isNFT)
		txData.RealTokenID = realTokenID
		if tx.GetVersion() == 2 {
			txData.PubKeyReceivers = append(txData.PubKeyReceivers, pubkey)
		}
		txDataList = append(txDataList, *txData)
	}

	coinV1AlreadyWrite := []shared.CoinDataV1{}

	if len(txDataList) > 0 {
		err = database.DBSaveTXs(txDataList)
		if err != nil {
			panic(err)
		}
	}
	if len(outCoinList) > 0 {
		err, coinV1AlreadyWrite = database.DBSaveCoins(outCoinList)
		if err != nil {
			panic(err)
		}
	}
	if len(keyImageList) > 0 {
		err = database.DBSaveUsedKeyimage(keyImageList)
		if err != nil {
			panic(err)
		}
	}

	if len(coinV1PubkeyInfo) > 0 {
		if len(coinV1AlreadyWrite) > 0 {
			for _, v := range coinV1AlreadyWrite {
				publicKeyStr := v.CoinPubkey
				coinInfo := coinV1PubkeyInfo[publicKeyStr][v.TokenID]
				coinInfo.Total -= 1
				coinV1PubkeyInfo[publicKeyStr][v.TokenID] = coinInfo
			}
		}
		err = database.DBUpdateCoinV1PubkeyInfo(coinV1PubkeyInfo)
		if err != nil {
			panic(err)
		}
	}

	statePrefix := fmt.Sprintf("coin-processed-%v", blk.Header.ShardID)
	err = Localnode.GetUserDatabase().Put([]byte(statePrefix), []byte(fmt.Sprintf("%v", blk.Header.Height)), nil)
	if err != nil {
		panic(err)
	}
	blockProcessedLock.Lock()
	blockProcessed[shardID] = blk.Header.Height
	blockProcessedLock.Unlock()
	log.Printf("finish processing coin for block %v shard %v in %v\n", blk.GetHeight(), shardID, time.Since(startTime))
}
