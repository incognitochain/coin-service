package trade

var currentState State

func StartProcessor() {
	go startProcessHistory()
}
