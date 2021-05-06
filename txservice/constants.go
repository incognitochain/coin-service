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
