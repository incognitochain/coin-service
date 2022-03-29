package coordinator

var state CoordinatorState

func init() {
	state.ConnectedServices = make(map[string]ServiceConn)
}

func StartCoordinator() {

}
