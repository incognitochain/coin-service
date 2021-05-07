package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/transaction"
)

var txTopic *pubsub.Topic
var statusTopic *pubsub.Topic

func main() {
	log.Println("initiating tx-service...")
	if err := ConnectDB(); err != nil {
		panic(err)
	}
	err := startPubsubClient()
	if err != nil {
		panic(err)
	}
	switch MODE {
	case PUSHMODE:
		pushMode()
	case BROADCASTMODE:
		broadcastMode()
	case STATUSMODE:
		statusMode()
	}
	select {}
}

func pushMode() {
	var err error
	txTopic, err = startPubsubTopic(TX_TOPIC)
	if err != nil {
		panic(err)
	}
	r := gin.Default()
	r.Use(gzip.Gzip(gzip.DefaultCompression))
	r.GET("/gettxstatus", APIGetTxStatus)
	r.POST("/pushtx", APIPushTx)

	err = r.Run("0.0.0.0:" + PORT)
	if err != nil {
		panic(err)
	}
}

func broadcastMode() {
	var err error
	txTopic, err = startPubsubTopic(TX_TOPIC)
	if err != nil {
		panic(err)
	}
	statusTopic, err = startPubsubTopic(TXSTATUS_TOPIC)
	if err != nil {
		panic(err)
	}
	// sub := psclient.Subscription(TX_SUBID)
	var sub *pubsub.Subscription
	sub, err = psclient.CreateSubscription(context.Background(), TX_SUBID,
		pubsub.SubscriptionConfig{Topic: txTopic})
	if err != nil {
		sub = psclient.Subscription(TX_SUBID)
	}
	log.Println("sub.ID()", sub.ID())
	ctx := context.Background()
	err = sub.Receive(ctx, func(ctx context.Context, m *pubsub.Message) {
		// log.Println(m.Data)
		rawTxBytes, _, err := base58.Base58Check{}.Decode(string(m.Data))
		if err != nil {
			log.Println(err)
			m.Ack()
			return
		}
		// Unmarshal from json data to object tx
		tx, err := transaction.NewTransactionFromJsonBytes(rawTxBytes)
		if err != nil {
			log.Println(err)
			m.Ack()
			return
		}
		errBrc := broadcastToFullNode(string(m.Data))
		txStatus := txStatusBroadcasted
		errBrcStr := ""
		_ = errBrc
		// if errBrc != nil {
		// 	log.Println(errBrc)
		// 	errBrcStr = errBrc.Error()
		// 	// txStatus = txStatusRetry
		// 	txStatus = txStatusFailed
		// 	m.Nack()
		// } else {
		m.Ack()
		// }
		txdata := NewTxData(tx.Hash().String(), string(m.Data), txStatus, errBrcStr)
		err = saveTx(*txdata)
		if err != nil {
			log.Println(err)
			return
		}
		if txStatus == txStatusBroadcasted {
			msg := &pubsub.Message{
				Attributes: map[string]string{
					"txhash": tx.Hash().String(),
				},
				Data: []byte(txStatus),
			}
			_, err = statusTopic.Publish(ctx, msg).Get(ctx)
			if err != nil {
				log.Println(err)
				return
			}
		}
		log.Println("Broadcast success")
	})
	if err != nil {
		log.Println(err)
	}
}

func statusMode() {
	var err error
	statusTopic, err = startPubsubTopic(TXSTATUS_TOPIC)
	if err != nil {
		panic(err)
	}
	var sub *pubsub.Subscription
	sub, err = psclient.CreateSubscription(context.Background(), TXSTATUS_SUBID,
		pubsub.SubscriptionConfig{Topic: statusTopic})
	if err != nil {
		log.Println(err)
		sub = psclient.Subscription(TXSTATUS_SUBID)
	}
	ctx := context.Background()
	err = sub.Receive(ctx, func(ctx context.Context, m *pubsub.Message) {
		fmt.Println(m.Data)
		if string(m.Data) == txStatusBroadcasted {
			txhash := m.Attributes["txhash"]
			log.Println(txhash, m.Attributes)
			inBlock, err := getTxStatusFullNode(txhash)
			if err != nil {
				log.Println(err)
				time.Sleep(4 * time.Second)
				m.Nack()
			}
			if inBlock {
				err = updateTxStatus(txhash, txStatusSuccess, "")
				if err != nil {
					log.Println(err)
					m.Nack()
				}
			} else {
				time.Sleep(4 * time.Second)
				m.Nack()
			}
		}
		m.Ack()
	})
	if err != nil {
		log.Println(err)
	}

	interval := time.NewTicker(40 * time.Second)
	for {
		<-interval.C
		txList, err := getAllFailedTx()
		if err != nil {
			log.Println(err)
			continue
		}
		log.Println(txList)
		// notifyViaSlack() or something
	}
}

func APIGetTxStatus(c *gin.Context) {
	txhash := c.Query("txhash")
	if txhash == "" {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("invalid txhash")))
		return
	}
	txData, err := getTx(txhash)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	respond := APIRespond{
		Result: txData.Status,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func APIPushTx(c *gin.Context) {
	type PushTxRequest struct {
		TxRaw string
	}
	var req PushTxRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	if req.TxRaw == "" {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("invalid txhash")))
		return
	}
	rawTxBytes, _, err := base58.Base58Check{}.Decode(req.TxRaw)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("invalid txhash")))
		return
	}
	// Unmarshal from json data to object tx))
	tx, err := transaction.NewTransactionFromJsonBytes(rawTxBytes)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	ctx := context.Background()
	msg := &pubsub.Message{
		Attributes: map[string]string{
			"txhash": tx.Hash().String(),
		},
		Data: []byte(req.TxRaw),
	}
	msgID, err := txTopic.Publish(ctx, msg).Get(ctx)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}
	log.Println("publish msgID:", msgID)

	respond := APIRespond{
		Result: "ok",
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func broadcastToFullNode(tx string) error {
	requestBody, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "1.0",
		"method":  "sendtransaction",
		"params":  []interface{}{tx},
		"id":      1,
	})
	if err != nil {
		return err
	}
	body, err := sendRequest(requestBody)
	if err != nil {
		return err
	}
	type ErrMsg struct {
		Code       int
		Message    string
		StackTrace string
	}

	resp := struct {
		Result map[string]interface{}
		Error  *ErrMsg
	}{}
	err = json.Unmarshal(body, &resp)

	if resp.Error != nil && resp.Error.StackTrace != "" {
		return errors.New(resp.Error.StackTrace)
	}

	if err != nil {
		return err
	}
	return nil
}

func getTxStatusFullNode(txhash string) (bool, error) {
	requestBody, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "1.0",
		"method":  "gettransactionbyhash",
		"params":  []interface{}{txhash},
		"id":      1,
	})
	if err != nil {
		return false, err
	}
	body, err := sendRequest(requestBody)
	if err != nil {
		return false, err
	}
	type ErrMsg struct {
		Code       int
		Message    string
		StackTrace string
	}

	resp := struct {
		Result map[string]interface{}
		Error  *ErrMsg
	}{}
	err = json.Unmarshal(body, &resp)

	if resp.Error != nil && resp.Error.StackTrace != "" {
		return false, errors.New(resp.Error.StackTrace)
	}

	if err != nil {
		return false, err
	}
	isInBlock := resp.Result["IsInBlock"].(bool)
	return isInBlock, nil

}
func sendRequest(requestBody []byte) ([]byte, error) {
	resp, err := http.Post(FULLNODE, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func buildGinErrorRespond(err error) *APIRespond {
	errStr := err.Error()
	respond := APIRespond{
		Result: nil,
		Error:  &errStr,
	}
	return &respond
}

type APIRespond struct {
	Result interface{}
	Error  *string
}
