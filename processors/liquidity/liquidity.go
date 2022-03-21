package liquidity

func StartProcessor() {
	go startProcessHistory()
	startAnalytic()
}
