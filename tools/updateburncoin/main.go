package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
)

func main() {
	argDBName := flag.String("dbname", "", "set dbname")
	argMongo := flag.String("mongo", "", "set mongo url")
	err := database.ConnectDB(*argDBName, *argMongo)
	if err != nil {
		panic(err)
	}
	start := time.Now()
	filter := bson.M{"coinpubkey": bson.M{operator.Eq: "1y4gnYS1Ns2K7BjQTjgfZ5nTR8JZMkMJ3CTGMj2Pk7CQkSTFgA"}}
	update := bson.M{
		"$set": bson.M{"otasecret": "1y4gnYS1Ns2K7BjQTjgfZ5nTR8JZMkMJ3CTGMj2Pk7CQkSTFgA"},
	}
	_, err = mgm.Coll(&shared.CoinData{}).UpdateMany(context.Background(), filter, update)
	if err != nil {
		panic(err)
	}
	log.Println("finish update burn coins in", time.Since(start))
}
