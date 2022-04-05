package otaindexer

import (
	"github.com/incognitochain/coin-service/coordinator"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/incognitokey"
)

type OTAkeyInfo struct {
	ShardID int
	Pubkey  string
	OTAKey  string
	keyset  *incognitokey.KeySet
	KeyInfo *shared.KeyInfoData
}

type OTAAssignRequest struct {
	Key     *shared.SubmittedOTAKeyData
	FromNow bool
	Respond chan error
}

type worker struct {
	ID          string
	Heartbeat   int64
	OTAAssigned int
	readCh      chan []byte
	writeCh     chan []byte
	closeCh     chan struct{}
}

type WorkerOTACmd struct {
	Action string
	Key    OTAkeyInfo
}

type CoordinatorState struct {
	coordinatorConn *coordinator.ServiceConn
	pauseService    bool
	serviceStatus   string
}
