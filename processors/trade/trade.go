package trade

func StartProcessor() {
	go startProcessHistory()
	startAnalytic()
}
