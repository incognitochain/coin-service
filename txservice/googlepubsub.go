package main

import (
	"context"
	"log"
	"sync"

	"cloud.google.com/go/pubsub"
)

var (
	psclient *pubsub.Client
	// Messages received by this instance.
	messagesMu sync.Mutex
	messages   []string
)

const maxMessages = 10

func startPubsubClient() error {
	ctx := context.Background()

	client, err := pubsub.NewClient(ctx, GGC_PROJECT)
	if err != nil {
		log.Fatal(err)
	}
	psclient = client
	return nil
}

func startPubsubTopic(topicName string) (*pubsub.Topic, error) {
	ctx := context.Background()

	topic := psclient.Topic(topicName)

	// Create the topic if it doesn't exist.
	exists, err := topic.Exists(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if !exists {
		log.Printf("Topic %v doesn't exist - creating it", topicName)
		_, err = psclient.CreateTopic(ctx, topicName)
		if err != nil {
			log.Fatal(err)
		}
	}
	return topic, nil
}
