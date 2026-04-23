package api

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	service "github.com/alireza0/s-ui/src/backend/internal/domain/services"
	database "github.com/alireza0/s-ui/src/backend/internal/infra/db"
	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
	logger "github.com/alireza0/s-ui/src/backend/internal/infra/logging"
	"github.com/alireza0/s-ui/src/backend/internal/shared/util"

	"github.com/gin-gonic/gin"
)

type apiUserService interface {
	GetUsers() (*[]model.User, error)
	Login(username string, password string, remoteIP string) (string, error)
	ChangePass(id string, oldPass string, newUser string, newPass string) error
	LoadTokens() ([]byte, error)
	GetUserTokens(username string) (*[]model.Tokens, error)
	AddToken(username string, expiry int64, desc string) (string, error)
	DeleteToken(id string) error
	IsFirstUser(username string) (bool, error)
}

type ApiService struct {
	service.SettingService
	service.UserService
	service.ConfigService
	service.ClientService
	service.TlsService
	service.InboundService
	service.OutboundService
	service.EndpointService
	service.ServicesService
	service.PanelService
	service.StatsService
	service.ServerService
	service.DomainHintService
	userService apiUserService
}

type authState struct {
	Username string `json:"username"`
	IsAdmin  bool   `json:"isAdmin"`
}

func (a *ApiService) LoadData(c *gin.Context) {
	data, err := a.getData(c)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonObj(c, data, nil)
}

func (a *ApiService) getData(c *gin.Context) (interface{}, error) {
	data := make(map[string]interface{}, 0)
	lu := c.Query("lu")
	isUpdated, err := a.ConfigService.CheckChanges(lu)
	if err != nil {
		return "", err
	}
	onlines, err := a.StatsService.GetOnlines()

	sysInfo := a.ServerService.GetSingboxInfo()
	if sysInfo["running"] == false {
		logs := a.ServerService.GetLogs("1", "debug")
		if len(logs) > 0 {
			data["lastLog"] = logs[0]
		}
	}

	if err != nil {
		return "", err
	}
	if isUpdated {
		config, err := a.SettingService.GetConfig()
		if err != nil {
			return "", err
		}
		clients, err := a.ClientService.GetAll()
		if err != nil {
			return "", err
		}
		tlsConfigs, err := a.TlsService.GetAll()
		if err != nil {
			return "", err
		}
		inbounds, err := a.InboundService.GetAll()
		if err != nil {
			return "", err
		}
		outbounds, err := a.OutboundService.GetAll()
		if err != nil {
			return "", err
		}
		endpoints, err := a.EndpointService.GetAll()
		if err != nil {
			return "", err
		}
		services, err := a.ServicesService.GetAll()
		if err != nil {
			return "", err
		}
		subURI, err := a.SettingService.GetFinalSubURI(getHostname(c))
		if err != nil {
			return "", err
		}
		trafficAge, err := a.SettingService.GetTrafficAge()
		if err != nil {
			return "", err
		}
		data["config"] = json.RawMessage(config)
		data["clients"] = clients
		data["tls"] = tlsConfigs
		data["inbounds"] = inbounds
		data["outbounds"] = outbounds
		data["endpoints"] = endpoints
		data["services"] = services
		data["subURI"] = subURI
		data["enableTraffic"] = trafficAge > 0
		data["onlines"] = onlines
	} else {
		data["onlines"] = onlines
	}

	return data, nil
}

func (a *ApiService) LoadPartialData(c *gin.Context, objs []string) error {
	data := make(map[string]interface{}, 0)
	id := c.Query("id")

	for _, obj := range objs {
		switch obj {
		case "inbounds":
			inbounds, err := a.InboundService.Get(id)
			if err != nil {
				return err
			}
			data[obj] = inbounds
		case "outbounds":
			outbounds, err := a.OutboundService.GetAll()
			if err != nil {
				return err
			}
			data[obj] = outbounds
		case "endpoints":
			endpoints, err := a.EndpointService.GetAll()
			if err != nil {
				return err
			}
			data[obj] = endpoints
		case "services":
			services, err := a.ServicesService.GetAll()
			if err != nil {
				return err
			}
			data[obj] = services
		case "tls":
			tlsConfigs, err := a.TlsService.GetAll()
			if err != nil {
				return err
			}
			data[obj] = tlsConfigs
		case "clients":
			clients, err := a.ClientService.Get(id)
			if err != nil {
				return err
			}
			data[obj] = clients
		case "config":
			config, err := a.SettingService.GetConfig()
			if err != nil {
				return err
			}
			data[obj] = json.RawMessage(config)
		case "settings":
			settings, err := a.SettingService.GetAllSetting()
			if err != nil {
				return err
			}
			data[obj] = settings
		}
	}

	jsonObj(c, data, nil)
	return nil
}

func (a *ApiService) GetUsers(c *gin.Context) {
	users, err := a.getUserService().GetUsers()
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonObj(c, *users, nil)
}

func (a *ApiService) GetAuthState(c *gin.Context) {
	username := GetLoginUser(c)
	isAdmin, err := a.getUserService().IsFirstUser(username)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}

	jsonObj(c, authState{
		Username: username,
		IsAdmin:  isAdmin,
	}, nil)
}

func (a *ApiService) getUserService() apiUserService {
	if a.userService != nil {
		return a.userService
	}

	return &a.UserService
}

func (a *ApiService) GetSettings(c *gin.Context) {
	data, err := a.SettingService.GetAllSetting()
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonObj(c, data, err)
}

func (a *ApiService) GetStats(c *gin.Context) {
	resource := c.Query("resource")
	tag := c.Query("tag")
	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil {
		limit = 100
	}
	data, err := a.StatsService.GetStats(resource, tag, limit)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonObj(c, data, err)
}

func (a *ApiService) GetStatus(c *gin.Context) {
	request := c.Query("r")
	result := a.ServerService.GetStatus(request)
	jsonObj(c, result, nil)
}

func (a *ApiService) GetOnlines(c *gin.Context) {
	onlines, err := a.StatsService.GetOnlines()
	jsonObj(c, onlines, err)
}

func (a *ApiService) GetLogs(c *gin.Context) {
	count := c.Query("c")
	level := c.Query("l")
	logs := a.ServerService.GetLogs(count, level)
	jsonObj(c, logs, nil)
}

func (a *ApiService) CheckChanges(c *gin.Context) {
	actor := c.Query("a")
	chngKey := c.Query("k")
	count := c.Query("c")
	changes := a.ConfigService.GetChanges(actor, chngKey, count)
	jsonObj(c, changes, nil)
}

func (a *ApiService) GetKeypairs(c *gin.Context) {
	kType := c.Query("k")
	options := c.Query("o")
	keypair := a.ServerService.GenKeypair(kType, options)
	jsonObj(c, keypair, nil)
}

func (a *ApiService) GetDomainHints(c *gin.Context) {
	force := c.Query("force") == "1"
	jsonObj(c, a.DomainHintService.GetCatalog(force), nil)
}

func (a *ApiService) GetDb(c *gin.Context) {
	exclude := c.Query("exclude")
	db, err := database.GetDb(exclude)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename=b-ui_"+time.Now().Format("20060102-150405")+".db")
	c.Writer.Write(db)
}

func (a *ApiService) postActions(c *gin.Context) (string, json.RawMessage, error) {
	var data map[string]json.RawMessage
	err := c.ShouldBind(&data)
	if err != nil {
		return "", nil, err
	}
	return string(data["action"]), data["data"], nil
}

func (a *ApiService) Login(c *gin.Context) {
	remoteIP := getRemoteIp(c)
	loginUser, err := a.getUserService().Login(c.Request.FormValue("user"), c.Request.FormValue("pass"), remoteIP)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}

	sessionMaxAge, err := a.SettingService.GetSessionMaxAge()
	if err != nil {
		logger.Infof("Unable to get session's max age from DB")
	}

	err = SetLoginUser(c, loginUser, sessionMaxAge)
	if err == nil {
		logger.Info("user ", loginUser, " login success")
	} else {
		logger.Warning("login failed: ", err)
	}

	jsonMsg(c, "", nil)
}

func (a *ApiService) ChangePass(c *gin.Context) {
	id := c.Request.FormValue("id")
	oldPass := c.Request.FormValue("oldPass")
	newUsername := c.Request.FormValue("newUsername")
	newPass := c.Request.FormValue("newPass")
	err := a.getUserService().ChangePass(id, oldPass, newUsername, newPass)
	if err == nil {
		logger.Info("change user credentials success")
		jsonMsg(c, "save", nil)
	} else {
		logger.Warning("change user credentials failed:", err)
		jsonMsg(c, "", err)
	}
}

func (a *ApiService) Save(c *gin.Context, loginUser string) {
	hostname := getHostname(c)
	obj := c.Request.FormValue("object")
	act := c.Request.FormValue("action")
	data := c.Request.FormValue("data")
	initUsers := c.Request.FormValue("initUsers")
	objs, err := a.ConfigService.Save(obj, act, json.RawMessage(data), initUsers, loginUser, hostname)
	if err != nil {
		jsonMsg(c, "save", err)
		return
	}
	err = a.LoadPartialData(c, objs)
	if err != nil {
		jsonMsg(c, obj, err)
	}
}

func (a *ApiService) RestartApp(c *gin.Context) {
	err := a.PanelService.RestartPanel(3)
	jsonMsg(c, "restartApp", err)
}

func (a *ApiService) GetPanelUpdate(c *gin.Context) {
	result, err := a.PanelService.GetUpdateInfo()
	jsonObj(c, result, err)
}

func (a *ApiService) StartPanelUpdate(c *gin.Context) {
	targetVersion := c.Request.FormValue("targetVersion")
	force := c.Request.FormValue("force") == "1" || strings.EqualFold(c.Request.FormValue("force"), "true")
	result, err := a.PanelService.StartUpdate(targetVersion, force)
	jsonObj(c, result, err)
}

func (a *ApiService) RestartSb(c *gin.Context) {
	err := a.ConfigService.RestartCore()
	jsonMsg(c, "restartSb", err)
}

func (a *ApiService) LinkConvert(c *gin.Context) {
	link := c.Request.FormValue("link")
	result, _, err := util.GetOutbound(link, 0)
	jsonObj(c, result, err)
}

func (a *ApiService) SubConvert(c *gin.Context) {
	link := c.Request.FormValue("link")
	result, err := util.GetExternalSub(link)
	jsonObj(c, result, err)
}

func (a *ApiService) ImportDb(c *gin.Context) {
	file, _, err := c.Request.FormFile("db")
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	defer file.Close()
	err = database.ImportDB(file)
	jsonMsg(c, "", err)
}

func (a *ApiService) Logout(c *gin.Context) {
	loginUser := GetLoginUser(c)
	if loginUser != "" {
		logger.Infof("user %s logout", loginUser)
	}
	ClearSession(c)
	jsonMsg(c, "", nil)
}

func (a *ApiService) LoadTokens() ([]byte, error) {
	return a.getUserService().LoadTokens()
}

func (a *ApiService) GetTokens(c *gin.Context) {
	loginUser := GetLoginUser(c)
	tokens, err := a.getUserService().GetUserTokens(loginUser)
	jsonObj(c, tokens, err)
}

func (a *ApiService) AddToken(c *gin.Context) {
	loginUser := GetLoginUser(c)
	expiry := c.Request.FormValue("expiry")
	expiryInt, err := strconv.ParseInt(expiry, 10, 64)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	desc := c.Request.FormValue("desc")
	token, err := a.getUserService().AddToken(loginUser, expiryInt, desc)
	jsonObj(c, token, err)
}

func (a *ApiService) DeleteToken(c *gin.Context) {
	tokenId := c.Request.FormValue("id")
	err := a.getUserService().DeleteToken(tokenId)
	jsonMsg(c, "", err)
}

func (a *ApiService) GetSingboxConfig(c *gin.Context) {
	rawConfig, err := a.ConfigService.GetConfig("")
	if err != nil {
		c.Status(400)
		c.Writer.WriteString(err.Error())
		return
	}
	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "attachment; filename=config_"+time.Now().Format("20060102-150405")+".json")
	c.Writer.Write(*rawConfig)
}

func (a *ApiService) GetCheckOutbound(c *gin.Context) {
	tag := c.Query("tag")
	link := c.Query("link")
	result := a.ConfigService.CheckOutbound(tag, link)
	jsonObj(c, result, nil)
}
