package apiservice

import (
	"errors"
	"strconv"
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

func calcAMPRate(virtA, virtB, sellAmount float64) float64 {
	var result float64
	k := virtA * virtB
	result = virtB - (k / (virtA + sellAmount))
	return result / sellAmount
}
