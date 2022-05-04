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
	"strconv"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/transaction"
	"github.com/kamva/mgm/v3"
)

var txTopic *pubsub.Topic
var statusTopic *pubsub.Topic

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in f", r)
		}
		time.Sleep(5 * time.Second)
		main()
	}()
	log.Println("initiating tx-service...")
	if err := ConnectDB(); err != nil {
		panic(err)
	}
	err := startPubsubClient()
	if err != nil {
		panic(err)
	}
	common.MaxShardNumber, err = strconv.Atoi(SHARDNUMBER)
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
	r.GET("/retrievetx", APIRetrieveTx)
	r.GET("/health", APIHealthCheck)

	err = r.Run("0.0.0.0:" + PORT)
	if err != nil {
		panic(err)
	}
}

func broadcastMode() {

	r := gin.Default()
	r.Use(gzip.Gzip(gzip.DefaultCompression))
	r.GET("/health", APIHealthCheck)
	go func() {
		err := r.Run("0.0.0.0:" + PORT)
		if err != nil {
			panic(err)
		}
	}()
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
		// var tx transaction.Tx
		tx, err := transaction.NewTransactionFromJsonBytes(rawTxBytes)
		// err = json.Unmarshal(rawTxBytes, &tx)
		isToken := false
		if err != nil {
			log.Println(err)
			tx, err = transaction.NewTransactionTokenFromJsonBytes(rawTxBytes)
			if err != nil {
				log.Println(err)
				m.Ack()
				return
			}
			isToken = true
		}

		errBrc := broadcastToFullNode(string(m.Data), isToken)
		txStatus := txStatusBroadcasted
		errBrcStr := ""
		// _ = errBrc
		if errBrc != nil {
			// log.Println(errBrc)
			errBrcStr = errBrc.Error()
			// txStatus = txStatusRetry
			txStatus = txStatusFailed
		}
		m.Ack()
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
		log.Fatalln(err)
	}
}

func statusMode() {
	r := gin.Default()
	r.Use(gzip.Gzip(gzip.DefaultCompression))
	r.GET("/health", APIHealthCheck)

	go func() {
		err := r.Run("0.0.0.0:" + PORT)
		if err != nil {
			panic(err)
		}
	}()
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
				if time.Since(m.PublishTime) > 30*time.Minute {
					err = updateTxStatus(txhash, txStatusFailed, "")
					if err != nil {
						time.Sleep(4 * time.Second)
						m.Nack()
						return
					}
					m.Ack()
					return
				}
				time.Sleep(4 * time.Second)
				m.Nack()
				return
			}
			if inBlock {
				err = updateTxStatus(txhash, txStatusSuccess, "")
				if err != nil {
					time.Sleep(4 * time.Second)
					m.Nack()
					return
				}
			} else {
				if time.Since(m.PublishTime) > 30*time.Minute {
					err = updateTxStatus(txhash, txStatusFailed, "")
					if err != nil {
						time.Sleep(4 * time.Second)
						m.Nack()
						return
					}
					m.Ack()
					return
				}
				time.Sleep(4 * time.Second)
				m.Nack()
				return
			}
		}
		m.Ack()
	})
	if err != nil {
		log.Fatalln(err)
	}

	go func() {
		//re
		for {
			txList, err := getAllPendingTx()
			if err != nil {
				log.Println(err)
				continue
			}
			for _, v := range txList {
				if time.Since(v.CreatedAt) > 5*time.Minute {
					txhash := v.TxHash
					inBlock, err := getTxStatusFullNode(txhash)
					if err != nil {
						log.Println(err)
						if time.Since(v.CreatedAt) > 30*time.Minute {
							err = updateTxStatus(txhash, txStatusFailed, "timeout")
							if err != nil {
								log.Println(err)
							}
							continue
						}
					}
					if inBlock {
						err = updateTxStatus(txhash, txStatusSuccess, "")
						if err != nil {
							log.Println(err)
							continue
						}
					} else {
						if time.Since(v.CreatedAt) > 30*time.Minute {
							err = updateTxStatus(txhash, txStatusFailed, "timeout")
							if err != nil {
								log.Println(err)
							}
							continue
						}
					}
				}
			}
			time.Sleep(20 * time.Second)
		}
	}()

	interval2 := time.NewTicker(40 * time.Second)
	for {
		<-interval2.C
		// txList, err := getAllFailedTx()
		// if err != nil {
		// 	log.Println(err)
		// 	continue
		// }
		// log.Println(txList)
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
		errStr := err.Error()
		respond := APIRespond{
			Result: TXSTATUS_UNKNOWN,
			ErrMsg: &errStr,
		}
		c.JSON(http.StatusOK, respond)
		return
	}

	status := TXSTATUS_UNKNOWN
	var errStr *string
	switch txData.Status {
	case txStatusSuccess:
		status = TXSTATUS_SUCCESS
	case txStatusFailed:
		status = TXSTATUS_FAILED
		errStr = &txData.Error
	case txStatusBroadcasted, txStatusRetry:
		status = TXSTATUS_PENDING
	}

	respond := APIRespond{
		Result: status,
		ErrMsg: errStr,
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
	// var tx transaction.Tx
	// err = json.Unmarshal(rawTxBytes, &tx)
	if err != nil {
		tx, err = transaction.NewTransactionTokenFromJsonBytes(rawTxBytes)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
	}
	txHash := tx.Hash().String()
	ctx := context.Background()
	msg := &pubsub.Message{
		Attributes: map[string]string{
			"txhash": txHash,
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
		Result: txHash,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func APIRetrieveTx(c *gin.Context) {
	txhash := c.Query("txhash")
	if txhash == "" {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("invalid txhash")))
		return
	}
	txData, err := getTx(txhash)
	if err != nil {
		errStr := err.Error()
		respond := APIRespond{
			Result: TXSTATUS_UNKNOWN,
			ErrMsg: &errStr,
		}
		c.JSON(http.StatusOK, respond)
		return
	}

	respond := APIRespond{
		Result: txData.Raw,
	}
	c.JSON(http.StatusOK, respond)
}

func APIHealthCheck(c *gin.Context) {
	status := shared.HEALTH_STATUS_OK
	mongoStatus := shared.MONGO_STATUS_OK
	_, cd, _, _ := mgm.DefaultConfigs()
	err := cd.Ping(context.Background(), nil)
	if err != nil {
		status = shared.HEALTH_STATUS_NOK
		mongoStatus = shared.MONGO_STATUS_NOK
	}
	c.JSON(http.StatusOK, gin.H{
		"status": status,
		"mongo":  mongoStatus,
	})
}

func broadcastToFullNode(tx string, isToken bool) error {
	methodName := "sendtransaction"
	if isToken {
		methodName = "sendrawprivacycustomtokentransaction"
	}
	requestBody, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "1.0",
		"method":  methodName,
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
	ErrMsg *string
}
