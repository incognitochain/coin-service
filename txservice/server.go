package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"cloud.google.com/go/pubsub"
	devframework "github.com/0xkumi/incognito-dev-framework"
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
	r := gin.Default()
	r.Use(gzip.Gzip(gzip.DefaultCompression))
	r.GET("/gettxstatus", APIGetTxStatus)
	r.POST("/pushtx", APIPushTx)

	err := r.Run("0.0.0.0:" + PORT)
	if err != nil {
		panic(err)
	}
	txTopic, err = startPubsubTopic(TX_TOPIC)
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
	sub := psclient.Subscription(TX_SUBID)
	// sub, err := psclient.CreateSubscription(context.Background(), TX_SUBID,
	// 	pubsub.SubscriptionConfig{Topic: txTopic})
	// if err != nil {
	// 	panic(err)
	// }
	ctx := context.Background()
	err = sub.Receive(ctx, func(ctx context.Context, m *pubsub.Message) {
		log.Println(m.Data)
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
		if errBrc != nil {
			log.Println(errBrc)
			// txStatus = txStatusRetry
			txStatus = txStatusFailed
			m.Nack()
		} else {
			m.Ack()
		}
		txdata := NewTxData(tx.Hash().String(), string(m.Data), txStatus, errBrc.Error())
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
	sub := psclient.Subscription(TXSTATUS_SUBID)
	// sub, err := psclient.CreateSubscription(context.Background(), TXSTATUS_SUBID,
	// 	pubsub.SubscriptionConfig{Topic: statusTopic})
	// if err != nil {
	// 	panic(err)
	// }
	ctx := context.Background()
	err = sub.Receive(ctx, func(ctx context.Context, m *pubsub.Message) {
		fmt.Println(m.Data)
		if string(m.Data) == txStatusBroadcasted {
			txhash := m.Attributes["txhash"]
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
	// Unmarshal from json data to object tx
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
	rpc := devframework.NewRPCClient(FULLNODE)
	_, err := rpc.API_SendRawTransaction(tx)
	if err != nil {
		return err
	}
	return nil
}

func getTxStatusFullNode(txhash string) (bool, error) {
	rpc := devframework.NewRPCClient(FULLNODE)
	result, err := rpc.API_GetTransactionHash(txhash)
	if err != nil {
		return false, err
	}
	if result.IsInBlock {
		return true, nil
	}
	return false, nil
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
