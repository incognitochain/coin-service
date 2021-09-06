package apiservice

import (
	"errors"
	"sync"

	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/transaction"
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
