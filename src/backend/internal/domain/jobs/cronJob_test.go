package cronjob

import "testing"

func TestScheduledCronJobsIncludeAutoPanelUpdateChecksEveryThreeMinutes(t *testing.T) {
	jobs := scheduledCronJobs(0)
	foundPanelStatusReport := false
	foundAutoUpdateCheck := false

	for _, job := range jobs {
		if _, ok := job.job.(*ClusterPanelStatusReportJob); ok {
			foundPanelStatusReport = true
			if job.spec != "@every 1m" {
				t.Fatalf("expected panel status report every 1m, got %q", job.spec)
			}
		}
		if _, ok := job.job.(*ClusterPanelAutoUpdateCheckJob); ok {
			foundAutoUpdateCheck = true
			if job.spec != "@every 3m" {
				t.Fatalf("expected auto panel update check every 3m, got %q", job.spec)
			}
		}
	}

	if !foundPanelStatusReport {
		t.Fatal("expected node panel status report job to remain scheduled")
	}
	if !foundAutoUpdateCheck {
		t.Fatal("expected automatic panel update check job to be scheduled")
	}
}
