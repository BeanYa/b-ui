package app

import (
	"context"
	"fmt"
	"log"

	"github.com/BeanYa/b-ui/src/backend/internal/domain/config"
	"github.com/BeanYa/b-ui/src/backend/internal/domain/core"
	cronjob "github.com/BeanYa/b-ui/src/backend/internal/domain/jobs"
	service "github.com/BeanYa/b-ui/src/backend/internal/domain/services"
	"github.com/BeanYa/b-ui/src/backend/internal/http/sub"
	database "github.com/BeanYa/b-ui/src/backend/internal/infra/db"
	logger "github.com/BeanYa/b-ui/src/backend/internal/infra/logging"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/web"

	"github.com/op/go-logging"
)

type APP struct {
	service.SettingService
	configService *service.ConfigService
	webServer     *web.Server
	subServer     *sub.Server
	cronJob       *cronjob.CronJob
	logger        *logging.Logger
	core          *core.Core
}

func NewApp() *APP {
	return &APP{}
}

func (a *APP) Init() error {
	log.Printf("%v %v", config.GetName(), config.GetVersion())

	a.initLog()

	err := database.InitDB(config.GetDBPath())
	if err != nil {
		return err
	}
	if err := a.applyStartupAdminCredentials(); err != nil {
		return err
	}

	// Init Setting
	a.SettingService.GetAllSetting()
	if loc, err := a.SettingService.GetTimeLocation(); err == nil {
		logger.SetClusterLogLocation(loc)
	}

	// One-shot retag: migrate legacy prefix/suffix-named domain inbounds to
	// the new segment-based slugs. Guarded by the namingRetagDone setting so
	// it runs exactly once per database.
	a.runNamingRetagOnce()

	a.core = core.NewCore()

	a.cronJob = cronjob.NewCronJob()
	a.webServer = web.NewServer()
	a.subServer = sub.NewServer()

	a.configService = service.NewConfigService(a.core)

	return nil
}

// runNamingRetagOnce recomputes domain inbound slugs/remarks once after the
// DB is migrated, guarded by the "namingRetagDone" setting flag.
func (a *APP) runNamingRetagOnce() {
	done, err := a.SettingService.GetBool("namingRetagDone")
	if err != nil {
		logger.Warning("naming retag guard read failed:", err)
		return
	}
	if done {
		return
	}
	svc := service.NewClusterDomainInboundService(service.ClusterDomainInboundServiceOptions{
		DB: database.GetDB(),
	})
	if err := svc.RetagAllDomainInbounds(context.Background()); err != nil {
		logger.Warning("naming retag failed:", err)
		return
	}
	if err := a.SettingService.SetBool("namingRetagDone", true); err != nil {
		logger.Warning("naming retag flag persist failed:", err)
	}
}

func (a *APP) applyStartupAdminCredentials() error {
	credentials := config.GetStartupAdminCredentials()
	if credentials.Username == "" && credentials.Password == "" {
		return nil
	}
	if credentials.Username == "" || credentials.Password == "" {
		return fmt.Errorf("both BUI_DEFAULT_ADMIN_USERNAME and BUI_DEFAULT_ADMIN_PASSWORD are required when setting startup admin credentials")
	}

	userService := service.UserService{}
	return userService.UpdateFirstUser(credentials.Username, credentials.Password)
}

func (a *APP) Start() error {
	loc, err := a.SettingService.GetTimeLocation()
	if err != nil {
		return err
	}
	logger.SetClusterLogLocation(loc)

	trafficAge, err := a.SettingService.GetTrafficAge()
	if err != nil {
		return err
	}

	err = a.cronJob.Start(loc, trafficAge)
	if err != nil {
		return err
	}

	err = a.webServer.Start()
	if err != nil {
		return err
	}

	err = a.subServer.Start()
	if err != nil {
		return err
	}

	err = a.configService.StartCore()
	if err != nil {
		logger.Error(err)
	}

	return nil
}

func (a *APP) Stop() {
	a.cronJob.Stop()
	err := a.subServer.Stop()
	if err != nil {
		logger.Warning("stop Sub Server err:", err)
	}
	err = a.webServer.Stop()
	if err != nil {
		logger.Warning("stop Web Server err:", err)
	}
	err = a.configService.StopCore()
	if err != nil {
		logger.Warning("stop Core err:", err)
	}
}

func (a *APP) initLog() {
	switch config.GetLogLevel() {
	case config.Debug:
		logger.InitLogger(logging.DEBUG)
	case config.Info:
		logger.InitLogger(logging.INFO)
	case config.Warn:
		logger.InitLogger(logging.WARNING)
	case config.Error:
		logger.InitLogger(logging.ERROR)
	default:
		log.Fatal("unknown log level:", config.GetLogLevel())
	}
	logger.InitClusterLogger()
}

func (a *APP) RestartApp() {
	a.Stop()
	a.Start()
}

func (a *APP) GetCore() *core.Core {
	return a.core
}
