package main

const (
	PUSHMODE      = "push"
	BROADCASTMODE = "broadcast"
	STATUSMODE    = "status"
)

const (
	txStatusSuccess     = "success"
	txStatusFailed      = "failed"
	txStatusBroadcasted = "broadcast"
	txStatusNew         = "new"
	txStatusRetry       = "retrying"
)

const (
	maxTxRetryTime = 30
)

const (
	TXSTATUS_UNKNOWN = iota
	TXSTATUS_FAILED
	TXSTATUS_PENDING
	TXSTATUS_SUCCESS
)
