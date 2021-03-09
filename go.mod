module main

go 1.13

require (
	github.com/0xkumi/incognito-dev-framework v0.0.0-20210126025433-6c9cd1a5c024
	github.com/gocql/gocql v0.0.0-20210303210847-f18e0979d243
	github.com/golang/snappy v0.0.3 // indirect
	github.com/gorilla/websocket v1.4.2
	github.com/incognitochain/incognito-chain v0.0.0-20210121082946-4a4818b0daca
	github.com/scylladb/gocqlx v1.5.0
	github.com/scylladb/gocqlx/v2 v2.3.0
	github.com/syndtr/goleveldb v1.0.0
)

replace github.com/incognitochain/incognito-chain => /home/lam/go/src/github.com/incognitochain/incognito-chain

replace github.com/0xkumi/incognito-dev-framework => /home/lam/go/src/github.com/0xkumi/incognito-dev-framework
