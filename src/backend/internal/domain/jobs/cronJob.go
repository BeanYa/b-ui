package cronjob

import (
	"time"

	"github.com/robfig/cron/v3"
)

type CronJob struct {
	cron *cron.Cron
}

type scheduledCronJob struct {
	spec string
	job  cron.Job
}

func NewCronJob() *CronJob {
	return &CronJob{}
}

func (c *CronJob) Start(loc *time.Location, trafficAge int) error {
	c.cron = cron.New(cron.WithLocation(loc), cron.WithSeconds())
	c.cron.Start()

	go func() {
		jobs := scheduledCronJobs(trafficAge)
		var panelStatusJob *ClusterPanelStatusReportJob
		for _, scheduled := range jobs {
			if job, ok := scheduled.job.(*ClusterPanelStatusReportJob); ok {
				panelStatusJob = job
			}
			c.cron.AddJob(scheduled.spec, scheduled.job)
		}
		if panelStatusJob != nil {
			go panelStatusJob.Run()
		}
	}()
	go NewDomainHintJob().Run()

	return nil
}

func (c *CronJob) Stop() {
	if c.cron != nil {
		c.cron.Stop()
	}
}

func scheduledCronJobs(trafficAge int) []scheduledCronJob {
	jobs := []scheduledCronJob{
		// Start stats job
		{spec: "@every 10s", job: NewStatsJob(trafficAge > 0)},
		// Start expiry job
		{spec: "@every 1m", job: NewDepleteJob()},
	}
	if trafficAge > 0 {
		// Start deleting old stats
		jobs = append(jobs, scheduledCronJob{spec: "@daily", job: NewDelStatsJob(trafficAge)})
	}
	jobs = append(jobs,
		// Start core if it is not running
		scheduledCronJob{spec: "@every 5s", job: NewCheckCoreJob()},
		// Refresh built-in TLS/Reality domain hints
		scheduledCronJob{spec: "@every 6h", job: NewDomainHintJob()},
		// database WAL checkpoint
		scheduledCronJob{spec: "@every 10m", job: NewWALCheckpointJob()},
		// node-side panel status reconciliation; this does not poll hub
		scheduledCronJob{spec: "@every 1m", job: NewClusterPanelStatusReportJob()},
		// low-frequency local-only peer reachability probing
		scheduledCronJob{spec: "@every 30s", job: NewClusterReachabilityProbeJob()},
		// periodic mesh ping based on per-domain ping policy
		scheduledCronJob{spec: "@every 30s", job: NewClusterMeshPingJob()},
		// cluster peer scheduled broadcasts
		scheduledCronJob{spec: "@every 30m", job: NewClusterPeerScheduleJob()},
	)
	return jobs
}
