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
	statusTopic, err = startPubsubTopic(TX_TOPIC + "_status")
	if err != nil {
		panic(err)
	}
	sub, err := psclient.CreateSubscription(context.Background(), "tx-broadcaster",
		pubsub.SubscriptionConfig{Topic: txTopic})
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	err = sub.Receive(ctx, func(ctx context.Context, m *pubsub.Message) {
		log.Println(m.Data)
		errBrc := broadcastToFullNode(string(m.Data))
		if errBrc != nil {
			log.Println(errBrc)
			m.Nack()
		} else {
			m.Ack()
		}
	})
	if err != nil {
		log.Println(err)
	}
}

func statusMode() {
	var err error
	statusTopic, err = startPubsubTopic(TX_TOPIC + "_status")
	if err != nil {
		panic(err)
	}
	sub, err := psclient.CreateSubscription(context.Background(), "tx-status",
		pubsub.SubscriptionConfig{Topic: statusTopic})
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	err = sub.Receive(ctx, func(ctx context.Context, m *pubsub.Message) {
		fmt.Println(m.Data)
		errBrc := broadcastToFullNode(string(m.Data))
		if errBrc != nil {
			log.Println(errBrc)
			m.Nack()
		} else {
			m.Ack()
		}
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
		// notifyViaSlack()
	}
}

func APIGetTxStatus(c *gin.Context) {
	txhash := c.Query("txhash")
	if txhash == "" {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("invalid txhash")))
		return
	}
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
		Attributes: make(map[string]string),
		Data:       []byte(req.TxRaw),
	}
	msg.Attributes["txhash"] = tx.Hash().String()
	if _, err := txTopic.Publish(ctx, msg).Get(ctx); err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}
	respond := APIRespond{
		Result: "ok",
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func broadcastToFullNode(tx string) error {
	rpc := devframework.NewRPCClient(FULLNODE)
	if rpc == nil {
		log.Fatalln("can't connect to fullnode")
	}
	_, err := rpc.API_SendRawTransaction(tx)
	if err != nil {
		return err
	}
	return nil
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
