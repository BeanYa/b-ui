package service

import "testing"

func TestNextScheduleRunDisablesOnceSchedule(t *testing.T) {
	schedule := PeerScheduleState{
		Kind:      "once",
		RunCount:  0,
		MaxRuns:   1,
		NextRunAt: 100,
	}

	next, enabled := NextPeerScheduleRun(schedule, 100)

	if enabled || next != 0 {
		t.Fatalf("expected disabled once schedule, got next=%d enabled=%v", next, enabled)
	}
}

func TestNextScheduleRunAdvancesInterval(t *testing.T) {
	schedule := PeerScheduleState{
		Kind:       "interval",
		RunCount:   2,
		MaxRuns:    5,
		IntervalMS: 1000,
		NextRunAt:  100,
	}

	next, enabled := NextPeerScheduleRun(schedule, 100)

	if !enabled || next != 1100 {
		t.Fatalf("expected next 1100 enabled, got next=%d enabled=%v", next, enabled)
	}
}

func TestNextScheduleRunDisablesExpiredInterval(t *testing.T) {
	schedule := PeerScheduleState{
		Kind:       "interval",
		RunCount:   0,
		IntervalMS: 1000,
		NextRunAt:  100,
		ExpiresAt:  100,
	}

	next, enabled := NextPeerScheduleRun(schedule, 100)

	if enabled || next != 0 {
		t.Fatalf("expected expired schedule disabled, got next=%d enabled=%v", next, enabled)
	}
}

func TestNextScheduleRunDisablesIntervalAtMaxRuns(t *testing.T) {
	schedule := PeerScheduleState{
		Kind:       "interval",
		RunCount:   2,
		MaxRuns:    3,
		IntervalMS: 1000,
		NextRunAt:  100,
	}

	next, enabled := NextPeerScheduleRun(schedule, 100)

	if enabled || next != 0 {
		t.Fatalf("expected maxed schedule disabled, got next=%d enabled=%v", next, enabled)
	}
}
