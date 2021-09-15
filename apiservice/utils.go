package apiservice

import (
	"errors"
	"sync"

	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/metadata"
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

func getTradeStatus(order *shared.TradeOrderData, limitOrderStatus *shared.LimitOrderStatus) (uint64, int, string, map[string]TradeWithdrawInfo, error) {
	var matchedAmount uint64
	var status string
	var sellTokenWDAmount uint64
	var sellTokenAmount int64
	withdrawTxs := make(map[string]TradeWithdrawInfo)

	if limitOrderStatus != nil {
		sellTokenAmount = int64(limitOrderStatus.Left)
	}

	for idx, v := range order.WithdrawTxs {
		withdrawInfo := TradeWithdrawInfo{
			Amount:  order.WithdrawAmount[idx],
			TokenID: order.WithdrawTokens[idx],
		}
		if len(order.WithdrawResponds) >= idx+1 {
			withdrawInfo.RespondTx = order.WithdrawResponds[idx]
			if order.WithdrawTokens[idx] == order.SellTokenID {
				sellTokenWDAmount += order.WithdrawAmount[idx]
			}
			withdrawInfo.Status = order.WithdrawStatus[idx]
		} else {
			withdrawInfo.Status = 0
		}
		withdrawTxs[v] = withdrawInfo
	}

	sellTokenAmount += int64(sellTokenWDAmount)
	matchedAmount = order.Amount - uint64(sellTokenAmount)
	if len(order.WithdrawTxs) > 0 {
		if len(order.WithdrawTxs) > len(order.WithdrawResponds) {
			status = "withdrawing"
		}
	} else {
		if sellTokenAmount == 0 {
			status = "success"
		} else {
			if len(order.RespondTxs) == 0 {
				status = "ongoing"
			} else {
				status = "reject"
			}
		}
	}
	statusCode := 0
	switch status {
	case "ongoing":
		statusCode = 0
	case "success":
		statusCode = 1
	case "reject":
		statusCode = 2
	case "withdrawing":
		statusCode = 3
	}
	return matchedAmount, statusCode, status, withdrawTxs, nil
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
		if otakeyOnly == true {
			return result, errors.New("key incorrect format")
		}
	} else {
		pubkey = wl.KeySet.OTAKey.GetPublicSpend().ToBytesS()
	}
	if wl.KeySet.PaymentAddress.GetPublicSpend().ToBytesS() == nil {
		return result, errors.New("key incorrect format")
	} else {
		pubkey = wl.KeySet.PaymentAddress.GetPublicSpend().ToBytesS()
	}
	result = base58.EncodeCheck(pubkey)
	return result, nil
}
