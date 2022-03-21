package shield

var analyticState State

func startAnalytic() {
	err := loadState(&analyticState, "shield-analytic")
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
}

func createAnalyticTable() error {

	return nil
}

func createContinuousView() error {
	return nil
}
