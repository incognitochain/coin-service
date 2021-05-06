package main

const (
	PUSHMODE      = "push"
	BROADCASTMODE = "broadcast"
	STATUSMODE    = "status"
)

const (
	txStatusSuccess = "success"
	txStatusFailed  = "failed"
	txStatusNew     = "new"
	txStatusRetry   = "retry"
)

const (
	maxTxRetryTime = 30
)
