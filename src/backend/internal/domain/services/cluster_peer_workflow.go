package service

func NextClusterPeerChainStep(route RoutePlan, currentStepID string, currentSucceeded bool) (RouteStep, bool) {
	if route.Mode != RouteModeChain {
		return RouteStep{}, false
	}

	for index, step := range route.Chain {
		if step.StepID != currentStepID {
			continue
		}
		if !currentSucceeded && !step.ContinueOnFailure {
			return RouteStep{}, false
		}
		if index+1 >= len(route.Chain) {
			return RouteStep{}, false
		}
		return route.Chain[index+1], true
	}

	return RouteStep{}, false
}
