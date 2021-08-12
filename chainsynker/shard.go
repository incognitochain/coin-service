package chainsynker

import (
	"encoding/base64"
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
	metadataPdexv3 "github.com/incognitochain/incognito-chain/metadata/pdexv3"
	"github.com/incognitochain/incognito-chain/privacy/coin"
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
	blockHeight := fmt.Sprintf("%v", blk.GetHeight())
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
	tradeRespondList := []shared.TradeOrderData{}
	tradeRequestList := []shared.TradeOrderData{}
	bridgeShieldRespondList := []shared.ShieldData{}
	txRespondMap := make(map[string][]struct {
		Amount   uint64
		TxID     string
		Locktime int64
	})
	contributionRespondList := []shared.ContributionData{}
	contributionWithdrawRepsondList := []shared.WithdrawContributionData{}
	contributionFeeWithdrawRepsondList := []shared.WithdrawContributionFeeData{}

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
		var outs []coin.Coin
		tokenIDStr := tx.GetTokenID().String()
		if tx.GetType() == common.TxCustomTokenPrivacyType || tx.GetType() == common.TxTokenConversionType {
			txToken := tx.(transaction.TransactionToken)
			if txToken.GetTxTokenData().TxNormal.GetProof() != nil {
				outs = txToken.GetTxTokenData().TxNormal.GetProof().GetOutputCoins()
				if isCoinV2Output && !tx.IsPrivacy() {
					txTokenData := transaction.GetTxTokenDataFromTransaction(tx)
					tokenIDStr = txTokenData.PropertyID.String()
				}
			}
		} else {
			outs = tx.GetProof().GetOutputCoins()
		}

		metaDataType := tx.GetMetadataType()
		switch metaDataType {
		case metadata.PDECrossPoolTradeRequestMeta, metadata.PDETradeRequestMeta, metadata.Pdexv3TradeRequestMeta, metadata.Pdexv3AddOrderRequestMeta:
			requestTx := tx.Hash().String()
			lockTime := tx.GetLockTime()
			buyToken := ""
			sellToken := ""
			poolID := ""
			pairID := ""
			rate := uint64(0)
			amount := uint64(0)
			nftID := ""
			switch metaDataType {
			case metadata.PDETradeRequestMeta:
				meta := tx.GetMetadata().(*metadata.PDECrossPoolTradeRequest)
				payment := meta.TraderAddressStr
				wl, err := wallet.Base58CheckDeserialize(payment)
				if err != nil {
					panic(err)
				}
				pubkey = base58.EncodeCheck(wl.KeySet.PaymentAddress.Pk)
				buyToken = meta.TokenIDToBuyStr
				sellToken = meta.TokenIDToSellStr
				pairID = meta.TokenIDToBuyStr + "-" + meta.TokenIDToSellStr
				rate = meta.MinAcceptableAmount / meta.SellAmount
				amount = meta.SellAmount
			case metadata.PDECrossPoolTradeRequestMeta:
				meta := tx.GetMetadata().(*metadata.PDETradeRequest)
				payment := meta.TraderAddressStr
				wl, err := wallet.Base58CheckDeserialize(payment)
				if err != nil {
					panic(err)
				}
				pubkey = base58.EncodeCheck(wl.KeySet.PaymentAddress.Pk)
				buyToken = meta.TokenIDToBuyStr
				sellToken = meta.TokenIDToSellStr
				pairID = meta.TokenIDToBuyStr + "-" + meta.TokenIDToSellStr
				rate = meta.MinAcceptableAmount / meta.SellAmount
				amount = meta.SellAmount
			case metadata.Pdexv3TradeRequestMeta:
				currentTrade, ok := tx.GetMetadata().(*metadataPdexv3.TradeRequest)
				if !ok {
					panic("invalid metadataPdexv3.TradeRequest")
				}
				amount = currentTrade.SellAmount
				sellToken = currentTrade.TokenToSell.String()
				rate = currentTrade.MinAcceptableAmount / currentTrade.SellAmount
			case metadata.Pdexv3AddOrderRequestMeta:
				item, ok := tx.GetMetadata().(*metadataPdexv3.AddOrderRequest)
				if !ok {
					panic("invalid metadataPdexv3.AddOrderRequest")
				}
				buyToken = item.TokenToBuy.String()
				sellToken = item.TokenToSell.String()
				amount = item.SellAmount
				pairID = item.PairID
				rate = item.MinAcceptableAmount / item.SellAmount
			}
			_ = buyToken
			trade := shared.NewTradeOrderData(requestTx, sellToken, poolID, pairID, "", nftID, nil, rate, amount, 0, lockTime)
			tradeRequestList = append(tradeRequestList, *trade)
		case metadata.PDECrossPoolTradeResponseMeta, metadata.PDETradeResponseMeta:
			status := ""
			switch metaDataType {
			case metadata.PDECrossPoolTradeResponseMeta:
				status = tx.GetMetadata().(*metadata.PDECrossPoolTradeResponse).TradeStatus
			case metadata.PDETradeResponseMeta:
				status = tx.GetMetadata().(*metadata.PDETradeResponse).TradeStatus
			}
			trade := shared.TradeOrderData{
				Status:     status,
				RespondTxs: []string{tx.Hash().String()},
			}
			tradeRespondList = append(tradeRespondList, trade)
		case metadata.Pdexv3WithdrawOrderRequestMeta:
			//TODO
		case metadata.Pdexv3WithdrawOrderResponseMeta:
			//TODO
		case metadata.IssuingResponseMeta, metadata.IssuingETHResponseMeta, metadata.IssuingBSCResponseMeta:
			requestTx := ""
			shieldType := "shield"
			bridge := ""
			isDecentralized := false
			switch metaDataType {
			case metadata.IssuingResponseMeta:
				requestTx = tx.GetMetadata().(*metadata.IssuingResponse).RequestedTxID.String()
				bridge = "btc"
			case metadata.IssuingETHResponseMeta, metadata.IssuingBSCResponseMeta:
				requestTx = tx.GetMetadata().(*metadata.IssuingEVMResponse).RequestedTxID.String()
				if metaDataType == metadata.IssuingETHResponseMeta {
					bridge = "eth"
				} else {
					bridge = "bsc"
				}
			}
			shielddata := shared.NewShieldData(requestTx, tx.Hash().String(), tokenIDStr, shieldType, bridge, "", isDecentralized, outs[0].GetValue(), beaconHeight)
			bridgeShieldRespondList = append(bridgeShieldRespondList, *shielddata)
		case metadata.PDEContributionResponseMeta:
			txRespondMap[tx.GetMetadata().(*metadata.PDEContributionResponse).RequestedTxID.String()] = append(txRespondMap[tx.GetMetadata().(*metadata.PDEContributionResponse).RequestedTxID.String()], struct {
				Amount   uint64
				TxID     string
				Locktime int64
			}{outs[0].GetValue(), txHash, tx.GetLockTime()})
		case metadata.PDEWithdrawalResponseMeta:
			txRespondMap[tx.GetMetadata().(*metadata.PDEWithdrawalResponse).RequestedTxID.String()] = append(txRespondMap[tx.GetMetadata().(*metadata.PDEWithdrawalResponse).RequestedTxID.String()], struct {
				Amount   uint64
				TxID     string
				Locktime int64
			}{outs[0].GetValue(), txHash, tx.GetLockTime()})
		case metadata.PDEFeeWithdrawalResponseMeta:
			txRespondMap[tx.GetMetadata().(*metadata.PDEFeeWithdrawalResponse).RequestedTxID.String()] = append(txRespondMap[tx.GetMetadata().(*metadata.PDEFeeWithdrawalResponse).RequestedTxID.String()], struct {
				Amount   uint64
				TxID     string
				Locktime int64
			}{outs[0].GetValue(), txHash, tx.GetLockTime()})
		case metadata.BurningRequestMeta, metadata.BurningRequestMetaV2, metadata.BurningForDepositToSCRequestMeta, metadata.BurningForDepositToSCRequestMetaV2, metadata.BurningPBSCRequestMeta:
			burningReqAction := tx.GetMetadata().(*metadata.BurningRequest)
			realTokenID = burningReqAction.TokenID.String()
			pubkey = base58.EncodeCheck(burningReqAction.BurnerAddress.GetPublicSpend().ToBytesS())
		case metadata.ContractingRequestMeta:
			contractingReqAction := tx.GetMetadata().(*metadata.ContractingRequest)
			realTokenID = contractingReqAction.TokenID.String()
			pubkey = base58.EncodeCheck(contractingReqAction.BurnerAddress.GetPublicSpend().ToBytesS())

			// Pdexv3AddLiquidityResponseMeta
		}

		mtd := ""
		if tx.GetMetadata() != nil {
			mtdBytes, err := json.Marshal(tx.GetMetadata())
			if err != nil {
				panic(err)
			}
			mtd = string(mtdBytes)
		}

		txData := shared.NewTxData(tx.GetLockTime(), shardID, int(tx.GetVersion()), blockHash, blockHeight, tokenID, txHash, tx.GetType(), string(txBytes), strconv.Itoa(metaDataType), mtd, txKeyImages, pubkeyReceivers)
		txData.RealTokenID = realTokenID
		if tx.GetVersion() == 2 {
			txData.PubKeyReceivers = append(txData.PubKeyReceivers, pubkey)
		}
		txDataList = append(txDataList, *txData)
	}

	beaconInsts := [][]string{}
	if blk.Header.Height != 1 {
		beaconInsts, err = extractInstructionFromBeaconBlocks(blk.Header.PreviousBlockHash.Bytes(), blk.Header.BeaconHeight)
		if err != nil {
			panic(err)
		}
	}

	withdrawProcessed := make(map[string]struct{})
	for _, inst := range beaconInsts {
		metaType, err := strconv.Atoi(inst[0])
		if err != nil {
			continue
		}
		switch metaType {
		case metadata.PDEContributionMeta, metadata.PDEPRVRequiredContributionRequestMeta:
			status := inst[2]
			contentBytes := []byte(inst[3])
			var contrData *shared.ContributionData
			if inst[2] == common.PDEContributionRefundChainStatus {
				var refundContribution metadata.PDERefundContribution
				err := json.Unmarshal(contentBytes, &refundContribution)
				if err != nil {
					panic(err)
				}
				reqTxID := refundContribution.TxReqID.String()
				if refundContribution.ShardID != byte(shardID) {
					log.Println("refundContribution.ShardID != shardID")
					continue
				}
				contrData = shared.NewContributionData(reqTxID, txRespondMap[reqTxID][0].TxID, status, refundContribution.PDEContributionPairID, refundContribution.TokenIDStr, refundContribution.ContributorAddressStr, refundContribution.ContributedAmount, 0, blk.GetHeight())
				contributionRespondList = append(contributionRespondList, *contrData)
			} else if inst[2] == common.PDEContributionMatchedNReturnedChainStatus {
				var matchedNReturnedContribution metadata.PDEMatchedNReturnedContribution
				txRespond := ""
				err := json.Unmarshal(contentBytes, &matchedNReturnedContribution)
				if err != nil {
					panic(err)
				}
				if matchedNReturnedContribution.ShardID != byte(shardID) {
					log.Println("matchedNReturnedContribution.ShardID != byte(shardID)")
					continue
				}
				reqTxID := matchedNReturnedContribution.TxReqID.String()
				if matchedNReturnedContribution.ReturnedContributedAmount == 0 {
					log.Println("matchedNReturnedContribution.ReturnedContributedAmount == 0")
				} else {
					txRespond = txRespondMap[reqTxID][0].TxID
				}

				contrData = shared.NewContributionData(reqTxID, txRespond, status, matchedNReturnedContribution.PDEContributionPairID, matchedNReturnedContribution.TokenIDStr, matchedNReturnedContribution.ContributorAddressStr, matchedNReturnedContribution.ActualContributedAmount, matchedNReturnedContribution.ReturnedContributedAmount, blk.GetHeight())
				contributionRespondList = append(contributionRespondList, *contrData)
			} else if inst[2] == common.PDEContributionMatchedChainStatus {
				var matchedContribution metadata.PDEMatchedContribution
				err := json.Unmarshal(contentBytes, &matchedContribution)
				if err != nil {
					panic(err)
				}
				wl, err := wallet.Base58CheckDeserialize(matchedContribution.ContributorAddressStr)
				if err != nil {
					panic(err)
				}
				pk := wl.KeySet.PaymentAddress.Pk
				if common.GetShardIDFromLastByte(pk[len(pk)-1]) != byte(shardID) {
					log.Println("matchedContribution.ShardID != byte(shardID)")
					continue
				}
				reqTxID := matchedContribution.TxReqID.String()
				contrData = shared.NewContributionData(reqTxID, "", status, matchedContribution.PDEContributionPairID, matchedContribution.TokenIDStr, matchedContribution.ContributorAddressStr, matchedContribution.ContributedAmount, 0, blk.GetHeight())
				contributionRespondList = append(contributionRespondList, *contrData)
			} else if inst[2] == common.PDEContributionWaitingChainStatus {
				var waitingContribution metadata.PDEWaitingContribution
				err := json.Unmarshal(contentBytes, &waitingContribution)
				if err != nil {
					panic(err)
				}
				wl, err := wallet.Base58CheckDeserialize(waitingContribution.ContributorAddressStr)
				if err != nil {
					panic(err)
				}
				pk := wl.KeySet.PaymentAddress.Pk
				if common.GetShardIDFromLastByte(pk[len(pk)-1]) != byte(shardID) {
					log.Println("matchedContribution.ShardID != byte(shardID)")
					continue
				}
				reqTxID := waitingContribution.TxReqID.String()
				contrData = shared.NewContributionData(reqTxID, "", status, waitingContribution.PDEContributionPairID, waitingContribution.TokenIDStr, waitingContribution.ContributorAddressStr, waitingContribution.ContributedAmount, 0, blk.GetHeight())
				contributionRespondList = append(contributionRespondList, *contrData)
			}
		case metadata.PDEWithdrawalRequestMeta:
			status := inst[2]
			if inst[2] == common.PDEWithdrawalAcceptedChainStatus {
				contentBytes := []byte(inst[3])
				var wdAcceptedContent metadata.PDEWithdrawalAcceptedContent
				err := json.Unmarshal(contentBytes, &wdAcceptedContent)
				if err != nil {
					panic(err)
				}
				if wdAcceptedContent.ShardID != byte(shardID) {
					log.Println("wdAcceptedContent.ShardID != shardID")
					continue
				}
				if _, ok := withdrawProcessed[wdAcceptedContent.TxReqID.String()]; ok {
					continue
				}
				txRespond := []string{txRespondMap[wdAcceptedContent.TxReqID.String()][0].TxID, txRespondMap[wdAcceptedContent.TxReqID.String()][1].TxID}
				wdData := shared.NewWithdrawContributionData(wdAcceptedContent.TxReqID.String(), status, wdAcceptedContent.PairToken1IDStr, wdAcceptedContent.PairToken2IDStr, wdAcceptedContent.WithdrawerAddressStr, txRespond, txRespondMap[wdAcceptedContent.TxReqID.String()][0].Amount, txRespondMap[wdAcceptedContent.TxReqID.String()][1].Amount, txRespondMap[wdAcceptedContent.TxReqID.String()][0].Locktime)
				contributionWithdrawRepsondList = append(contributionWithdrawRepsondList, *wdData)
				withdrawProcessed[wdAcceptedContent.TxReqID.String()] = struct{}{}
			}
		case metadata.PDEFeeWithdrawalRequestMeta:
			if inst[2] == common.PDEFeeWithdrawalAcceptedChainStatus {
				contentBytes, err := base64.StdEncoding.DecodeString(inst[3])
				if err != nil {
					panic(err)
				}
				var pdeFeeWithdrawalRequestAction metadata.PDEFeeWithdrawalRequestAction
				err = json.Unmarshal(contentBytes, &pdeFeeWithdrawalRequestAction)
				if err != nil {
					panic(err)
				}
				if pdeFeeWithdrawalRequestAction.ShardID != byte(shardID) {
					log.Println("pdeFeeWithdrawalRequestAction.ShardID != shardID")
					continue
				}
				wdData := shared.NewWithdrawContributionFeeData(pdeFeeWithdrawalRequestAction.TxReqID.String(), txRespondMap[pdeFeeWithdrawalRequestAction.TxReqID.String()][0].TxID, inst[2], pdeFeeWithdrawalRequestAction.Meta.WithdrawalToken1IDStr, pdeFeeWithdrawalRequestAction.Meta.WithdrawalToken2IDStr, pdeFeeWithdrawalRequestAction.Meta.WithdrawerAddressStr, pdeFeeWithdrawalRequestAction.Meta.WithdrawalFeeAmt, txRespondMap[pdeFeeWithdrawalRequestAction.TxReqID.String()][0].Locktime)
				contributionFeeWithdrawRepsondList = append(contributionFeeWithdrawRepsondList, *wdData)
			}
		}
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

	if len(tradeRequestList) > 0 {
		err = database.DBSaveTradeOrder(tradeRequestList)
		if err != nil {
			panic(err)
		}
	}

	if len(tradeRespondList) > 0 {
		err = database.DBSaveTradeOrder(tradeRespondList)
		if err != nil {
			panic(err)
		}
	}

	if len(bridgeShieldRespondList) > 0 {
		err = database.DBSaveTxShield(bridgeShieldRespondList)
		if err != nil {
			panic(err)
		}
	}
	if len(contributionRespondList) > 0 {
		err = database.DBSavePDEContribute(contributionRespondList)
		if err != nil {
			panic(err)
		}
	}

	if len(contributionWithdrawRepsondList) > 0 {
		err = database.DBSavePDEWithdraw(contributionWithdrawRepsondList)
		if err != nil {
			panic(err)
		}
	}

	if len(contributionFeeWithdrawRepsondList) > 0 {
		err = database.DBSavePDEWithdrawFee(contributionFeeWithdrawRepsondList)
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
	log.Printf("finish processing coin for block %v shard %v in %v\n", blk.GetHeight(), shardID, time.Since(startTime))
}
