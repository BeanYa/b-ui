package service

type PeerScheduleState struct {
	Kind       string
	RunCount   int
	MaxRuns    int
	IntervalMS int64
	NextRunAt  int64
	ExpiresAt  int64
}

func NextPeerScheduleRun(schedule PeerScheduleState, now int64) (int64, bool) {
	if schedule.ExpiresAt > 0 && now >= schedule.ExpiresAt {
		return 0, false
	}

	nextRunCount := schedule.RunCount + 1
	if schedule.MaxRuns > 0 && nextRunCount >= schedule.MaxRuns {
		return 0, false
	}

	switch schedule.Kind {
	case "once":
		return 0, false
	case "interval":
		return now + schedule.IntervalMS, true
	default:
		return 0, false
	}
}

type ClusterPeerScheduleService struct{}

func (s ClusterPeerScheduleService) RunDueSchedules() error {
	return nil
}
