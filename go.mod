module main

go 1.13

require (
	github.com/0xkumi/incognito-dev-framework v0.0.0-20210222031614-567de9ac8b64
	github.com/gorilla/websocket v1.4.2
	github.com/incognitochain/incognito-chain v0.0.0-20210303110621-bb9b06d611f5
	github.com/kamva/mgm/v3 v3.1.0
	github.com/scylladb/gocqlx v1.5.0
	github.com/syndtr/goleveldb v1.0.0
	go.mongodb.org/mongo-driver v1.5.0
)

replace github.com/incognitochain/incognito-chain => /home/lam/go/src/github.com/incognitochain/incognito-chain

replace github.com/0xkumi/incognito-dev-framework => /home/lam/go/src/github.com/0xkumi/incognito-dev-framework
