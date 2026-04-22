package cronjob

import (
	"time"

	"github.com/robfig/cron/v3"
)

type CronJob struct {
	cron *cron.Cron
}

func NewCronJob() *CronJob {
	return &CronJob{}
}

func (c *CronJob) Start(loc *time.Location, trafficAge int) error {
	c.cron = cron.New(cron.WithLocation(loc), cron.WithSeconds())
	c.cron.Start()

	go func() {
		// Start stats job
		c.cron.AddJob("@every 10s", NewStatsJob(trafficAge > 0))
		// Start expiry job
		c.cron.AddJob("@every 1m", NewDepleteJob())
		// Start deleting old stats
		if trafficAge > 0 {
			c.cron.AddJob("@daily", NewDelStatsJob(trafficAge))
		}
		// Start core if it is not running
		c.cron.AddJob("@every 5s", NewCheckCoreJob())
		// Refresh built-in TLS/Reality domain hints
		c.cron.AddJob("@every 6h", NewDomainHintJob())
		// database WAL checkpoint
		c.cron.AddJob("@every 10m", NewWALCheckpointJob())
		// cluster version polling
		c.cron.AddJob("@every 30m", NewClusterVersionPollJob())
	}()
	go NewDomainHintJob().Run()

	return nil
}

func (c *CronJob) Stop() {
	if c.cron != nil {
		c.cron.Stop()
	}
}
