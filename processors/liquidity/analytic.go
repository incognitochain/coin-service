package liquidity

import "time"

var analyticState State

func startAnalytic() {
	err := loadState(&analyticState, "liquidity-analytic")
	if err != nil {
		panic(err)
	}
	err = createAnalyticTable()
	if err != nil {
		panic(err)
	}

	err = createContinuousView()
	if err != nil {
		panic(err)
	}

	for {
		time.Sleep(5 * time.Second)
		startTime := time.Now()

	}
}

func createAnalyticTable() error {

	return nil
}

func createContinuousView() error {
	return nil
}
