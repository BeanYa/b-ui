package service

import (
	"encoding/json"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/BeanYa/b-ui/src/backend/internal/domain/config"
	database "github.com/BeanYa/b-ui/src/backend/internal/infra/db"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
	logger "github.com/BeanYa/b-ui/src/backend/internal/infra/logging"
	"github.com/BeanYa/b-ui/src/backend/internal/shared/util/common"

	"gorm.io/gorm"
)

var defaultConfig = `{
  "log": {
    "level": "info"
  },
  "dns": {
    "servers": [],
    "rules": []
  },
  "route": {
    "rules": [
		  {
        "action": "sniff"
      },
      {
        "protocol": [
          "dns"
        ],
        "action": "hijack-dns"
      }
    ]
  },
  "experimental": {}
}`

var defaultValueMap = map[string]string{
	"webListen":              "",
	"webDomain":              "",
	"webPort":                "2095",
	"secret":                 common.Random(32),
	"webCertFile":            "",
	"webKeyFile":             "",
	"webPath":                "/app/",
	"webURI":                 "",
	"webTerminalIdleTimeout": "300",
	"sessionMaxAge":          "0",
	"trafficAge":             "30",
	"timeLocation":           "Asia/Tehran",
	"tlsDomainHints":         "www.youtube.com\nwww.cloudflare.com\nwww.apple.com\nwww.microsoft.com\nwww.amazon.com\nwww.github.com\nwww.nvidia.com\nwww.adobe.com\nwww.bing.com\nwww.dropbox.com",
	"subListen":              "",
	"subPort":                "2096",
	"subPath":                "/sub/",
	"subDomain":              "",
	"subCertFile":            "",
	"subKeyFile":             "",
	"subUpdates":             "12",
	"subEncode":              "true",
	"subShowInfo":            "false",
	"subURI":                 "",
	"subJsonExt":             "",
	"subClashExt":            "",
	"config":                 defaultConfig,
	"version":                config.GetVersion(),
}

type SettingService struct {
}

func (s *SettingService) GetAllSetting() (*map[string]string, error) {
	db := database.GetDB()
	settings := make([]*model.Setting, 0)
	err := db.Model(model.Setting{}).Find(&settings).Error
	if err != nil {
		return nil, err
	}
	allSetting := map[string]string{}

	for _, setting := range settings {
		allSetting[setting.Key] = setting.Value
	}

	for key, defaultValue := range defaultValueMap {
		if _, exists := allSetting[key]; !exists {
			err = s.saveSetting(key, defaultValue)
			if err != nil {
				return nil, err
			}
			allSetting[key] = defaultValue
		}
	}

	// Due to security principles
	delete(allSetting, "secret")
	delete(allSetting, "config")
	delete(allSetting, "version")
	delete(allSetting, "webTerminalIdleTimeout")
	delete(allSetting, "webSSHIdleTimeout")

	return &allSetting, nil
}

func (s *SettingService) ResetSettings() error {
	db := database.GetDB()
	return db.Where("1 = 1").Delete(model.Setting{}).Error
}

func (s *SettingService) getSetting(key string) (*model.Setting, error) {
	db := database.GetDB()
	setting := &model.Setting{}
	err := db.Model(model.Setting{}).Where("key = ?", key).First(setting).Error
	if err != nil {
		return nil, err
	}
	return setting, nil
}

func (s *SettingService) getString(key string) (string, error) {
	setting, err := s.getSetting(key)
	if database.IsNotFound(err) {
		value, ok := defaultValueMap[key]
		if !ok {
			return "", common.NewErrorf("key <%v> not in defaultValueMap", key)
		}
		return value, nil
	} else if err != nil {
		return "", err
	}
	return setting.Value, nil
}

func (s *SettingService) saveSetting(key string, value string) error {
	setting, err := s.getSetting(key)
	db := database.GetDB()
	if database.IsNotFound(err) {
		return db.Create(&model.Setting{
			Key:   key,
			Value: value,
		}).Error
	} else if err != nil {
		return err
	}
	setting.Key = key
	setting.Value = value
	return db.Save(setting).Error
}

func (s *SettingService) setString(key string, value string) error {
	return s.saveSetting(key, value)
}

func (s *SettingService) getBool(key string) (bool, error) {
	str, err := s.getString(key)
	if err != nil {
		return false, err
	}
	return strconv.ParseBool(str)
}

// func (s *SettingService) setBool(key string, value bool) error {
// 	return s.setString(key, strconv.FormatBool(value))
// }

func (s *SettingService) getInt(key string) (int, error) {
	str, err := s.getString(key)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(str)
}

func (s *SettingService) setInt(key string, value int) error {
	return s.setString(key, strconv.Itoa(value))
}
func (s *SettingService) GetListen() (string, error) {
	return s.getString("webListen")
}

func (s *SettingService) GetWebDomain() (string, error) {
	return s.getString("webDomain")
}

func (s *SettingService) SetWebDomain(domain string) error {
	return s.setString("webDomain", domain)
}

func (s *SettingService) GetPort() (int, error) {
	return s.getInt("webPort")
}

func (s *SettingService) SetPort(port int) error {
	return s.setInt("webPort", port)
}

func (s *SettingService) GetCertFile() (string, error) {
	return s.getString("webCertFile")
}

func (s *SettingService) SetCertFile(path string) error {
	return s.setString("webCertFile", path)
}

func (s *SettingService) GetKeyFile() (string, error) {
	return s.getString("webKeyFile")
}

func (s *SettingService) SetKeyFile(path string) error {
	return s.setString("webKeyFile", path)
}

func (s *SettingService) GetWebPath() (string, error) {
	webPath, err := s.getString("webPath")
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(webPath, "/") {
		webPath = "/" + webPath
	}
	if !strings.HasSuffix(webPath, "/") {
		webPath += "/"
	}
	return webPath, nil
}

func (s *SettingService) SetWebPath(webPath string) error {
	if !strings.HasPrefix(webPath, "/") {
		webPath = "/" + webPath
	}
	if !strings.HasSuffix(webPath, "/") {
		webPath += "/"
	}
	return s.setString("webPath", webPath)
}

func (s *SettingService) GetSecret() ([]byte, error) {
	secret, err := s.getString("secret")
	if secret == defaultValueMap["secret"] {
		err := s.saveSetting("secret", secret)
		if err != nil {
			logger.Warning("save secret failed:", err)
		}
	}
	return []byte(secret), err
}

func (s *SettingService) GetSessionMaxAge() (int, error) {
	return s.getInt("sessionMaxAge")
}

func (s *SettingService) GetWebTerminalIdleTimeout() (time.Duration, error) {
	seconds, err := s.getTerminalIdleTimeoutSeconds()
	if err != nil {
		return 0, err
	}
	if seconds <= 0 {
		return 0, nil
	}
	return time.Duration(seconds) * time.Second, nil
}

func (s *SettingService) getTerminalIdleTimeoutSeconds() (int, error) {
	setting, err := s.getSetting("webTerminalIdleTimeout")
	if err == nil {
		return strconv.Atoi(setting.Value)
	}
	if err != nil && !database.IsNotFound(err) {
		return 0, err
	}

	legacySetting, legacyErr := s.getSetting("webSSHIdleTimeout")
	if legacyErr == nil {
		if saveErr := s.saveSetting("webTerminalIdleTimeout", legacySetting.Value); saveErr != nil {
			return 0, saveErr
		}
		return strconv.Atoi(legacySetting.Value)
	}
	if legacyErr != nil && !database.IsNotFound(legacyErr) {
		return 0, legacyErr
	}

	return strconv.Atoi(defaultValueMap["webTerminalIdleTimeout"])
}

func (s *SettingService) GetTrafficAge() (int, error) {
	return s.getInt("trafficAge")
}

func (s *SettingService) GetTimeLocation() (*time.Location, error) {
	l, err := s.getString("timeLocation")
	if err != nil {
		return nil, err
	}
	if runtime.GOOS == "windows" {
		l = "Local"
	}
	location, err := time.LoadLocation(l)
	if err != nil {
		defaultLocation := defaultValueMap["timeLocation"]
		logger.Errorf("location <%v> not exist, using default location: %v", l, defaultLocation)
		return time.LoadLocation(defaultLocation)
	}
	return location, nil
}

func (s *SettingService) GetTLSDomainHints() ([]string, error) {
	raw, err := s.getString("tlsDomainHints")
	if err != nil {
		return nil, err
	}

	items := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == ';'
	})
	uniq := make(map[string]struct{})
	domains := make([]string, 0, len(items))
	for _, item := range items {
		domain := strings.TrimSpace(item)
		if domain == "" {
			continue
		}
		if _, exists := uniq[domain]; exists {
			continue
		}
		uniq[domain] = struct{}{}
		domains = append(domains, domain)
	}

	return domains, nil
}

func (s *SettingService) GetSubListen() (string, error) {
	return s.getString("subListen")
}

func (s *SettingService) GetSubPort() (int, error) {
	return s.getInt("subPort")
}

func (s *SettingService) SetSubPort(subPort int) error {
	return s.setInt("subPort", subPort)
}

func (s *SettingService) GetSubPath() (string, error) {
	subPath, err := s.getString("subPath")
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(subPath, "/") {
		subPath = "/" + subPath
	}
	if !strings.HasSuffix(subPath, "/") {
		subPath += "/"
	}
	return subPath, nil
}

func (s *SettingService) SetSubPath(subPath string) error {
	if !strings.HasPrefix(subPath, "/") {
		subPath = "/" + subPath
	}
	if !strings.HasSuffix(subPath, "/") {
		subPath += "/"
	}
	return s.setString("subPath", subPath)
}

func (s *SettingService) GetSubDomain() (string, error) {
	return s.getString("subDomain")
}

func (s *SettingService) SetSubDomain(domain string) error {
	return s.setString("subDomain", domain)
}

func (s *SettingService) GetSubCertFile() (string, error) {
	return s.getString("subCertFile")
}

func (s *SettingService) SetSubCertFile(path string) error {
	return s.setString("subCertFile", path)
}

func (s *SettingService) GetSubKeyFile() (string, error) {
	return s.getString("subKeyFile")
}

func (s *SettingService) SetSubKeyFile(path string) error {
	return s.setString("subKeyFile", path)
}

func (s *SettingService) GetSubTLSFiles() (string, string, error) {
	subCertFile, err := s.getString("subCertFile")
	if err != nil {
		return "", "", err
	}
	subKeyFile, err := s.getString("subKeyFile")
	if err != nil {
		return "", "", err
	}
	webCertFile, err := s.getString("webCertFile")
	if err != nil {
		return "", "", err
	}
	webKeyFile, err := s.getString("webKeyFile")
	if err != nil {
		return "", "", err
	}

	certFile, keyFile := resolveSubTLSFiles(subCertFile, subKeyFile, webCertFile, webKeyFile)
	return certFile, keyFile, nil
}

func resolveSubTLSFiles(subCertFile string, subKeyFile string, webCertFile string, webKeyFile string) (string, string) {
	subCertFile = strings.TrimSpace(subCertFile)
	subKeyFile = strings.TrimSpace(subKeyFile)
	webCertFile = strings.TrimSpace(webCertFile)
	webKeyFile = strings.TrimSpace(webKeyFile)

	if subCertFile != "" && subKeyFile != "" {
		return subCertFile, subKeyFile
	}
	if webCertFile != "" && webKeyFile != "" {
		return webCertFile, webKeyFile
	}
	return "", ""
}

func (s *SettingService) GetSubUpdates() (int, error) {
	return s.getInt("subUpdates")
}

func (s *SettingService) GetSubEncode() (bool, error) {
	return s.getBool("subEncode")
}

func (s *SettingService) GetSubShowInfo() (bool, error) {
	return s.getBool("subShowInfo")
}

func (s *SettingService) GetSubURI() (string, error) {
	return s.getString("subURI")
}

func (s *SettingService) GetFinalSubURI(host string) (string, error) {
	allSetting, err := s.GetAllSetting()
	if err != nil {
		return "", err
	}
	SubURI := (*allSetting)["subURI"]
	if SubURI != "" {
		return SubURI, nil
	}
	protocol := "http"
	certFile, keyFile := resolveSubTLSFiles(
		(*allSetting)["subCertFile"],
		(*allSetting)["subKeyFile"],
		(*allSetting)["webCertFile"],
		(*allSetting)["webKeyFile"],
	)
	if certFile != "" && keyFile != "" {
		protocol = "https"
	}
	if (*allSetting)["subDomain"] != "" {
		host = (*allSetting)["subDomain"]
	}
	port := ":" + (*allSetting)["subPort"]
	if (port == "80" && protocol == "http") || (port == "443" && protocol == "https") {
		port = ""
	}
	return protocol + "://" + host + port + (*allSetting)["subPath"], nil
}

func (s *SettingService) GetConfig() (string, error) {
	return s.getString("config")
}

func (s *SettingService) SetConfig(config string) error {
	return s.setString("config", config)
}

func (s *SettingService) SaveConfig(tx *gorm.DB, config json.RawMessage) error {
	configs, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return tx.Model(model.Setting{}).Where("key = ?", "config").Update("value", string(configs)).Error
}

func (s *SettingService) Save(tx *gorm.DB, data json.RawMessage) error {
	var err error
	var settings map[string]string
	err = json.Unmarshal(data, &settings)
	if err != nil {
		return err
	}
	_, hasSubCertFile := settings["subCertFile"]
	_, hasSubKeyFile := settings["subKeyFile"]
	if hasSubCertFile && hasSubKeyFile && isSubTLSLinkedSetting(settings) {
		settings["subCertFile"] = ""
		settings["subKeyFile"] = ""
	}
	for key, obj := range settings {
		// Secure file existence check
		if obj != "" && (key == "webCertFile" ||
			key == "webKeyFile" ||
			key == "subCertFile" ||
			key == "subKeyFile") {
			err = s.fileExists(obj)
			if err != nil {
				return common.NewError(" -> ", obj, " is not exists")
			}
		}

		// Correct Pathes start and ends with `/`
		if key == "webPath" ||
			key == "subPath" {
			if !strings.HasPrefix(obj, "/") {
				obj = "/" + obj
			}
			if !strings.HasSuffix(obj, "/") {
				obj += "/"
			}
		}

		// Delete all stats if it is set to 0
		if key == "trafficAge" && obj == "0" {
			err = tx.Where("id > 0").Delete(model.Stats{}).Error
			if err != nil {
				return err
			}
		}
		err = tx.Model(model.Setting{}).Where("key = ?", key).Update("value", obj).Error
		if err != nil {
			return err
		}
	}
	return err
}

func isSubTLSLinkedSetting(settings map[string]string) bool {
	return strings.TrimSpace(settings["subCertFile"]) == "" || strings.TrimSpace(settings["subKeyFile"]) == ""
}

func (s *SettingService) GetSubJsonExt() (string, error) {
	return s.getString("subJsonExt")
}

func (s *SettingService) GetSubClashExt() (string, error) {
	return s.getString("subClashExt")
}

func (s *SettingService) fileExists(path string) error {
	_, err := os.Stat(path)
	return err
}
