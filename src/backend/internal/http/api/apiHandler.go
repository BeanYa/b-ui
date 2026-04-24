package api

import (
	"strings"

	service "github.com/alireza0/s-ui/src/backend/internal/domain/services"
	"github.com/alireza0/s-ui/src/backend/internal/shared/util/common"

	"github.com/gin-gonic/gin"
)

type APIHandler struct {
	ApiService
	apiv2                *APIv2Handler
	clusterService       clusterAPIService
	webSSHSessionFactory webSSHSessionFactory
}

func NewAPIHandler(g *gin.RouterGroup, a2 *APIv2Handler) {
	a := &APIHandler{
		apiv2:          a2,
		clusterService: &service.ClusterService{},
	}
	a.initRouter(g)
}

func (a *APIHandler) initRouter(g *gin.RouterGroup) {
	g.Use(func(c *gin.Context) {
		path := c.Request.URL.Path
		if !strings.HasSuffix(path, "login") && !strings.HasSuffix(path, "logout") {
			checkLogin(c)
		}
	})
	g.GET("/webssh/ws", a.handleWebSSH)
	g.POST("/cluster/register", a.registerCluster)
	g.GET("/cluster/operations/:id", a.getClusterOperation)
	g.GET("/cluster/domains", a.listClusterDomains)
	g.GET("/cluster/members", a.listClusterMembers)
	g.POST("/cluster/sync", a.manualClusterSync)
	g.DELETE("/cluster/domains/:id", a.leaveClusterDomain)
	g.DELETE("/cluster/members/:id", a.deleteClusterMember)
	g.POST("/:postAction", a.postHandler)
	g.GET("/:getAction", a.getHandler)
}

func (a *APIHandler) postHandler(c *gin.Context) {
	loginUser := GetLoginUser(c)
	action := c.Param("postAction")

	switch action {
	case "login":
		a.ApiService.Login(c)
	case "changePass":
		a.ApiService.ChangePass(c)
	case "save":
		a.ApiService.Save(c, loginUser)
	case "restartApp":
		a.ApiService.RestartApp(c)
	case "panelUpdate":
		a.ApiService.StartPanelUpdate(c)
	case "restartSb":
		a.ApiService.RestartSb(c)
	case "linkConvert":
		a.ApiService.LinkConvert(c)
	case "subConvert":
		a.ApiService.SubConvert(c)
	case "importdb":
		a.ApiService.ImportDb(c)
	case "addToken":
		a.ApiService.AddToken(c)
		a.apiv2.ReloadTokens()
	case "deleteToken":
		a.ApiService.DeleteToken(c)
		a.apiv2.ReloadTokens()
	default:
		jsonMsg(c, "failed", common.NewError("unknown action: ", action))
	}
}

func (a *APIHandler) getHandler(c *gin.Context) {
	action := c.Param("getAction")

	switch action {
	case "logout":
		a.ApiService.Logout(c)
	case "load":
		a.ApiService.LoadData(c)
	case "inbounds", "outbounds", "endpoints", "services", "tls", "clients", "config":
		err := a.ApiService.LoadPartialData(c, []string{action})
		if err != nil {
			jsonMsg(c, action, err)
		}
		return
	case "users":
		a.ApiService.GetUsers(c)
	case "authState":
		a.ApiService.GetAuthState(c)
	case "settings":
		a.ApiService.GetSettings(c)
	case "stats":
		a.ApiService.GetStats(c)
	case "status":
		a.ApiService.GetStatus(c)
	case "panelUpdate":
		a.ApiService.GetPanelUpdate(c)
	case "onlines":
		a.ApiService.GetOnlines(c)
	case "logs":
		a.ApiService.GetLogs(c)
	case "changes":
		a.ApiService.CheckChanges(c)
	case "keypairs":
		a.ApiService.GetKeypairs(c)
	case "domainHints":
		a.ApiService.GetDomainHints(c)
	case "getdb":
		a.ApiService.GetDb(c)
	case "tokens":
		a.ApiService.GetTokens(c)
	case "singbox-config":
		a.ApiService.GetSingboxConfig(c)
	case "checkOutbound":
		a.ApiService.GetCheckOutbound(c)
	default:
		jsonMsg(c, "failed", common.NewError("unknown action: ", action))
	}
}
