package coordinator

type CoordinatorCmd struct {
	Action string
	Data   string
}

type CoordinatorState struct {
	ConnectedServices map[string]ServiceConn
}

type ServiceConn struct {
	ServiceName string
	ID          string
	Heartbeat   int64
	OTAAssigned int
	readCh      chan []byte
	writeCh     chan []byte
	closeCh     chan struct{}
}
