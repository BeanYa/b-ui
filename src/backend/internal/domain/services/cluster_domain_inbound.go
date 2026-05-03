package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
	database "github.com/BeanYa/b-ui/src/backend/internal/infra/db"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/gorm"
)

type clusterDomainInboundSaver interface {
	Save(tx *gorm.DB, act string, data json.RawMessage, initUserIds string, hostname string) error
}

type clusterDomainInboundIdentity interface {
	GetOrCreate() (*model.ClusterLocalNode, error)
}

type clusterDomainInboundBroadcaster interface {
	BroadcastDomainInboundCreate(context.Context, *model.ClusterDomain, clustertypes.DomainInboundCreatePayload) error
}

type clusterDomainInboundReporter interface {
	ReportProxyConfigs(domainID uint)
}

type ClusterDomainInboundServiceOptions struct {
	DB                       *gorm.DB
	InboundSaver             clusterDomainInboundSaver
	Identity                 clusterDomainInboundIdentity
	Broadcaster              clusterDomainInboundBroadcaster
	Reporter                 clusterDomainInboundReporter
	PortAllocator            func() (int, error)
	TLSKeypairGenerator      func(serverName string) []string
	PanelCertificateProvider func() (DomainInboundPanelCertificateSettings, error)
	Now                      func() int64
}

type ClusterDomainInboundService struct {
	db                       *gorm.DB
	inboundSaver             clusterDomainInboundSaver
	identity                 clusterDomainInboundIdentity
	broadcaster              clusterDomainInboundBroadcaster
	reporter                 clusterDomainInboundReporter
	portAllocator            func() (int, error)
	tlsKeypairGenerator      func(serverName string) []string
	panelCertificateProvider func() (DomainInboundPanelCertificateSettings, error)
	now                      func() int64
}

type DomainInboundCreateResult struct {
	InboundID uint
	RequestID string
	Created   bool
}

type DomainInboundPanelCertificateSettings struct {
	WebDomain   string
	WebCertFile string
	WebKeyFile  string
}

func NewClusterDomainInboundService(opts ClusterDomainInboundServiceOptions) *ClusterDomainInboundService {
	s := &ClusterDomainInboundService{
		db:                       opts.DB,
		inboundSaver:             opts.InboundSaver,
		identity:                 opts.Identity,
		broadcaster:              opts.Broadcaster,
		reporter:                 opts.Reporter,
		portAllocator:            opts.PortAllocator,
		tlsKeypairGenerator:      opts.TLSKeypairGenerator,
		panelCertificateProvider: opts.PanelCertificateProvider,
		now:                      opts.Now,
	}
	if s.inboundSaver == nil {
		s.inboundSaver = &InboundService{}
	}
	if s.identity == nil {
		s.identity = &ClusterLocalIdentityService{}
	}
	if s.portAllocator == nil {
		s.portAllocator = allocateDomainInboundPort
	}
	if s.tlsKeypairGenerator == nil {
		s.tlsKeypairGenerator = (&ServerService{}).generateTLSKeyPair
	}
	if s.panelCertificateProvider == nil {
		s.panelCertificateProvider = defaultDomainInboundPanelCertificateSettings
	}
	if s.now == nil {
		s.now = func() int64 { return time.Now().Unix() }
	}
	return s
}

func defaultClusterDomainInboundOptions(broadcaster clusterDomainInboundBroadcaster, reporter clusterDomainInboundReporter) ClusterDomainInboundServiceOptions {
	return ClusterDomainInboundServiceOptions{
		DB:           database.GetDB(),
		InboundSaver: &InboundService{},
		Identity:     &ClusterLocalIdentityService{},
		Broadcaster:  broadcaster,
		Reporter:     reporter,
	}
}

func (s *ClusterDomainInboundService) ApplyDomainInboundCreate(ctx context.Context, domain *model.ClusterDomain, payload clustertypes.DomainInboundCreatePayload, source string, broadcast bool) (*DomainInboundCreateResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil {
		return nil, errors.New("cluster domain inbound service is required")
	}
	if s.db == nil {
		return nil, errors.New("cluster domain inbound db is required")
	}
	if s.inboundSaver == nil {
		return nil, errors.New("cluster domain inbound saver is required")
	}
	if s.identity == nil {
		return nil, errors.New("cluster domain inbound identity is required")
	}
	if domain == nil || strings.TrimSpace(domain.Domain) == "" || domain.Id == 0 {
		return nil, errors.New("local domain is required")
	}

	payload.RequestID = strings.TrimSpace(payload.RequestID)
	payload.DomainID = strings.TrimSpace(payload.DomainID)
	if payload.RequestID == "" {
		return nil, errors.New("request_id is required")
	}
	if payload.DomainID != "" && payload.DomainID != domain.Domain {
		return nil, fmt.Errorf("payload domain_id %q does not match local domain %q", payload.DomainID, domain.Domain)
	}
	if len(strings.TrimSpace(string(payload.Inbound))) == 0 {
		return nil, errors.New("inbound is required")
	}

	local, err := s.identity.GetOrCreate()
	if err != nil {
		return nil, err
	}
	nodeID := ""
	if local != nil {
		nodeID = local.NodeID
	}

	result := &DomainInboundCreateResult{RequestID: payload.RequestID}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var wrapper model.ClusterInbound
		err := tx.Where("domain_id = ? AND request_id = ?", domain.Id, payload.RequestID).First(&wrapper).Error
		if err == nil {
			result.InboundID = wrapper.InboundID
			result.Created = false
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		inboundJSON, tag, err := s.prepareDomainInboundJSON(tx, domain, payload, nodeID)
		if err != nil {
			return err
		}
		if err := s.inboundSaver.Save(tx, "new", inboundJSON, "", "ClusterDomainInbound"); err != nil {
			return err
		}

		var inbound model.Inbound
		if err := tx.Where("tag = ?", tag).First(&inbound).Error; err != nil {
			return err
		}
		now := s.now()
		wrapper = model.ClusterInbound{
			DomainID:  domain.Id,
			Domain:    domain.Domain,
			NodeID:    nodeID,
			MemberID:  nodeID,
			InboundID: inbound.Id,
			RequestID: payload.RequestID,
			Prefix:    payload.Prefix,
			Suffix:    payload.Suffix,
			Template:  strings.TrimSpace(payload.TLSTemplate),
			CreatedAt: now,
			UpdatedAt: now,
		}
		if source = strings.TrimSpace(source); source != "" {
			wrapper.MemberID = source
		}
		if err := tx.Create(&wrapper).Error; err != nil {
			return err
		}
		result.InboundID = inbound.Id
		result.Created = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result.Created && broadcast && s.broadcaster != nil {
		if err := s.broadcaster.BroadcastDomainInboundCreate(ctx, domain, payload); err != nil {
			return nil, err
		}
	}
	if result.Created && s.reporter != nil {
		s.reporter.ReportProxyConfigs(domain.Id)
	}
	return result, nil
}

func (s *ClusterDomainInboundService) HandleDomainInboundCreate(ctx context.Context, req clustertypes.ActionRequest, payload clustertypes.DomainInboundCreatePayload) (map[string]interface{}, error) {
	domain, err := (&dbClusterStore{}).GetDomainByName(req.Domain)
	if err != nil {
		return nil, err
	}
	result, err := s.ApplyDomainInboundCreate(ctx, domain, payload, req.SourceNodeID, true)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"inbound_id": result.InboundID,
		"request_id": result.RequestID,
		"created":    result.Created,
	}, nil
}

func (s *ClusterDomainInboundService) prepareDomainInboundJSON(tx *gorm.DB, domain *model.ClusterDomain, payload clustertypes.DomainInboundCreatePayload, nodeID string) (json.RawMessage, string, error) {
	var inbound map[string]interface{}
	if err := json.Unmarshal(payload.Inbound, &inbound); err != nil {
		return nil, "", err
	}
	inboundType := strings.TrimSpace(stringValue(inbound["type"]))
	if inboundType == "" {
		return nil, "", errors.New("inbound type is required")
	}
	baseTag := strings.TrimSpace(stringValue(inbound["tag"]))
	if baseTag == "" {
		baseTag = inboundType
	}
	tag := buildClusterInboundTag(payload.Prefix, baseTag, nodeID, payload.Suffix)

	delete(inbound, "id")
	delete(inbound, "tls_id")
	delete(inbound, "tls")
	delete(inbound, "out_json")
	delete(inbound, "addrs")
	delete(inbound, "users")
	inbound["type"] = inboundType
	inbound["tag"] = tag
	if strings.TrimSpace(stringValue(inbound["listen"])) == "" {
		inbound["listen"] = "::"
	}
	if err := validateDomainInboundListenPortLocalProvided(inbound["listen_port"]); err != nil {
		return nil, "", err
	}
	port, err := s.portAllocator()
	if err != nil {
		return nil, "", err
	}
	inbound["listen_port"] = port
	if err := rejectUnresolvedLocalProvided(inbound, "inbound"); err != nil {
		return nil, "", err
	}

	if payload.TLS != nil {
		tlsID, err := s.createDomainInboundTLS(tx, domain, tag, payload)
		if err != nil {
			return nil, "", err
		}
		inbound["tls_id"] = tlsID
	}

	data, err := json.Marshal(inbound)
	if err != nil {
		return nil, "", err
	}
	return data, tag, nil
}

func (s *ClusterDomainInboundService) createDomainInboundTLS(tx *gorm.DB, domain *model.ClusterDomain, tag string, payload clustertypes.DomainInboundCreatePayload) (uint, error) {
	server, err := rawObject(payload.TLS.Server)
	if err != nil {
		return 0, err
	}
	client, err := rawObject(payload.TLS.Client)
	if err != nil {
		return 0, err
	}
	if err := s.resolveDomainInboundTLSLocalProvided(domain, server, client); err != nil {
		return 0, err
	}
	switch strings.TrimSpace(payload.TLSTemplate) {
	case "standard", "hysteria2":
		if err := s.ensureGeneratedTLSKeypair(server); err != nil {
			return 0, err
		}
	case "reality":
		if err := ensureRealityKeypair(server, client, tag, s.now()); err != nil {
			return 0, err
		}
	}
	if err := rejectUnresolvedLocalProvided(server, "tls.server"); err != nil {
		return 0, err
	}
	if err := rejectUnresolvedLocalProvided(client, "tls.client"); err != nil {
		return 0, err
	}
	serverJSON, err := json.Marshal(server)
	if err != nil {
		return 0, err
	}
	clientJSON, err := json.Marshal(client)
	if err != nil {
		return 0, err
	}
	name := strings.TrimSpace(payload.TLS.Name)
	if name == "" {
		name = fmt.Sprintf("%s-tls-%d", tag, s.now())
	}
	tls := model.Tls{Name: name, Server: serverJSON, Client: clientJSON}
	if err := tx.Create(&tls).Error; err != nil {
		return 0, err
	}
	return tls.Id, nil
}

const (
	localProvidedKey                     = "LocalProvided"
	localProvidedDomainInboundListenPort = "DomainInboundListenPort"
	localProvidedDomainName              = "DomainName"
	localProvidedGeneratedTLSCertificate = "GeneratedTLSCertificate"
	localProvidedGeneratedTLSKey         = "GeneratedTLSKey"
	localProvidedPanelWebDomain          = "PanelWebDomain"
	localProvidedPanelWebCertFile        = "PanelWebCertFile"
	localProvidedPanelWebKeyFile         = "PanelWebKeyFile"
	localProvidedRealityPrivateKey       = "RealityPrivateKey"
	localProvidedRealityPublicKey        = "RealityPublicKey"
)

func defaultDomainInboundPanelCertificateSettings() (DomainInboundPanelCertificateSettings, error) {
	settings := &SettingService{}
	webDomain, err := settings.GetWebDomain()
	if err != nil {
		return DomainInboundPanelCertificateSettings{}, err
	}
	webCertFile, err := settings.GetCertFile()
	if err != nil {
		return DomainInboundPanelCertificateSettings{}, err
	}
	webKeyFile, err := settings.GetKeyFile()
	if err != nil {
		return DomainInboundPanelCertificateSettings{}, err
	}
	return DomainInboundPanelCertificateSettings{
		WebDomain:   webDomain,
		WebCertFile: webCertFile,
		WebKeyFile:  webKeyFile,
	}, nil
}

func validateDomainInboundListenPortLocalProvided(value interface{}) error {
	kind, ok := localProvidedKind(value)
	if !ok {
		return nil
	}
	if kind != localProvidedDomainInboundListenPort {
		return unsupportedLocalProvided("inbound.listen_port", kind)
	}
	return nil
}

func (s *ClusterDomainInboundService) resolveDomainInboundTLSLocalProvided(domain *model.ClusterDomain, server map[string]interface{}, client map[string]interface{}) error {
	var panelSettings *DomainInboundPanelCertificateSettings
	loadPanelSettings := func() (DomainInboundPanelCertificateSettings, error) {
		if panelSettings != nil {
			return *panelSettings, nil
		}
		settings, err := s.panelCertificateProvider()
		if err != nil {
			return DomainInboundPanelCertificateSettings{}, err
		}
		settings.WebDomain = strings.TrimSpace(settings.WebDomain)
		settings.WebCertFile = strings.TrimSpace(settings.WebCertFile)
		settings.WebKeyFile = strings.TrimSpace(settings.WebKeyFile)
		panelSettings = &settings
		return settings, nil
	}

	if kind, ok := localProvidedKind(server["server_name"]); ok {
		switch kind {
		case localProvidedDomainName:
			if domain != nil {
				server["server_name"] = strings.TrimSpace(domain.Domain)
			} else {
				server["server_name"] = ""
			}
		case localProvidedPanelWebDomain:
			settings, err := loadPanelSettings()
			if err != nil {
				return err
			}
			server["server_name"] = settings.WebDomain
		default:
			return unsupportedLocalProvided("tls.server.server_name", kind)
		}
	}

	if kind, ok := localProvidedKind(server["certificate_path"]); ok {
		if kind != localProvidedPanelWebCertFile {
			return unsupportedLocalProvided("tls.server.certificate_path", kind)
		}
		settings, err := loadPanelSettings()
		if err != nil {
			return err
		}
		if settings.WebCertFile == "" {
			return errors.New("panel TLS certificate is not configured")
		}
		server["certificate_path"] = settings.WebCertFile
	}
	if kind, ok := localProvidedKind(server["key_path"]); ok {
		if kind != localProvidedPanelWebKeyFile {
			return unsupportedLocalProvided("tls.server.key_path", kind)
		}
		settings, err := loadPanelSettings()
		if err != nil {
			return err
		}
		if settings.WebKeyFile == "" {
			return errors.New("panel TLS certificate key is not configured")
		}
		server["key_path"] = settings.WebKeyFile
	}

	if err := resolveRealityLocalProvided(server, client); err != nil {
		return err
	}
	return nil
}

func resolveRealityLocalProvided(server map[string]interface{}, client map[string]interface{}) error {
	if serverReality, ok := server["reality"].(map[string]interface{}); ok {
		if kind, ok := localProvidedKind(serverReality["private_key"]); ok {
			if kind != localProvidedRealityPrivateKey {
				return unsupportedLocalProvided("tls.server.reality.private_key", kind)
			}
			serverReality["private_key"] = ""
		}
	}
	if clientReality, ok := client["reality"].(map[string]interface{}); ok {
		if kind, ok := localProvidedKind(clientReality["public_key"]); ok {
			if kind != localProvidedRealityPublicKey {
				return unsupportedLocalProvided("tls.client.reality.public_key", kind)
			}
			delete(clientReality, "public_key")
		}
	}
	return nil
}

func localProvidedKind(value interface{}) (string, bool) {
	obj, ok := value.(map[string]interface{})
	if !ok {
		return "", false
	}
	raw, ok := obj[localProvidedKey]
	if !ok {
		return "", false
	}
	kind := strings.TrimSpace(stringValue(raw))
	return kind, kind != ""
}

func rejectUnresolvedLocalProvided(value interface{}, path string) error {
	if kind, ok := localProvidedKind(value); ok {
		return unsupportedLocalProvided(path, kind)
	}
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			nextPath := key
			if path != "" {
				nextPath = path + "." + key
			}
			if err := rejectUnresolvedLocalProvided(child, nextPath); err != nil {
				return err
			}
		}
	case []interface{}:
		for index, child := range typed {
			nextPath := fmt.Sprintf("%s[%d]", path, index)
			if err := rejectUnresolvedLocalProvided(child, nextPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func unsupportedLocalProvided(path string, kind string) error {
	return fmt.Errorf("unsupported LocalProvided %q at %s", kind, path)
}

func (s *ClusterDomainInboundService) ensureGeneratedTLSKeypair(server map[string]interface{}) error {
	if kind, ok := localProvidedKind(server["certificate"]); ok && kind != localProvidedGeneratedTLSCertificate {
		return unsupportedLocalProvided("tls.server.certificate", kind)
	}
	if kind, ok := localProvidedKind(server["key"]); ok && kind != localProvidedGeneratedTLSKey {
		return unsupportedLocalProvided("tls.server.key", kind)
	}
	if hasInlineTLSCertificate(server) {
		return nil
	}
	certPath := strings.TrimSpace(stringValue(server["certificate_path"]))
	keyPath := strings.TrimSpace(stringValue(server["key_path"]))
	if certPath != "" || keyPath != "" {
		if certPath == "" || keyPath == "" {
			return errors.New("tls certificate_path and key_path must both be set")
		}
		return nil
	}
	serverName := normalizeDomainInboundTLSServerName(stringValue(server["server_name"]))
	certificate, key, err := parseGeneratedTLSKeypair(s.tlsKeypairGenerator(serverName))
	if err != nil {
		return err
	}
	server["certificate"] = certificate
	server["key"] = key
	delete(server, "certificate_path")
	delete(server, "key_path")
	return nil
}

func hasInlineTLSCertificate(server map[string]interface{}) bool {
	return hasTLSValue(server["certificate"]) && hasTLSValue(server["key"])
}

func hasTLSValue(value interface{}) bool {
	if _, ok := localProvidedKind(value); ok {
		return false
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) != ""
	case []string:
		return len(v) > 0
	case []interface{}:
		return len(v) > 0
	default:
		return value != nil
	}
}

func normalizeDomainInboundTLSServerName(serverName string) string {
	serverName = strings.TrimSpace(serverName)
	if serverName == "" {
		return "''"
	}
	return serverName
}

func parseGeneratedTLSKeypair(lines []string) ([]string, []string, error) {
	certificate := make([]string, 0)
	key := make([]string, 0)
	activeBlock := ""
	for _, rawLine := range lines {
		line := strings.TrimRight(rawLine, "\r")
		if isDomainInboundPEMBoundary(line, "CERTIFICATE", "BEGIN") {
			activeBlock = "certificate"
			certificate = append(certificate, line)
			continue
		}
		if isDomainInboundPEMBoundary(line, "CERTIFICATE", "END") {
			certificate = append(certificate, line)
			activeBlock = ""
			continue
		}
		if strings.HasPrefix(line, "-----BEGIN ") && strings.HasSuffix(line, " PRIVATE KEY-----") {
			activeBlock = "key"
			key = append(key, line)
			continue
		}
		if strings.HasPrefix(line, "-----END ") && strings.HasSuffix(line, " PRIVATE KEY-----") {
			key = append(key, line)
			activeBlock = ""
			continue
		}
		switch activeBlock {
		case "certificate":
			certificate = append(certificate, line)
		case "key":
			key = append(key, line)
		}
	}
	if len(certificate) == 0 || len(key) == 0 {
		return nil, nil, errors.New("failed to generate TLS keypair")
	}
	return certificate, key, nil
}

func isDomainInboundPEMBoundary(line string, marker string, boundary string) bool {
	return strings.HasPrefix(line, "-----"+boundary+" ") && strings.HasSuffix(line, " "+marker+"-----")
}

func allocateDomainInboundPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, errors.New("allocated listener is not tcp")
	}
	return addr.Port, nil
}

func ensureRealityKeypair(server map[string]interface{}, client map[string]interface{}, tag string, now int64) error {
	serverReality := nestedObject(server, "reality")
	clientReality := nestedObject(client, "reality")
	privateValue := strings.TrimSpace(stringValue(serverReality["private_key"]))
	var publicValue string
	if privateValue == "" {
		privateKey, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return err
		}
		privateValue = privateKey.String()
		publicValue = privateKey.PublicKey().String()
		serverReality["private_key"] = privateValue
	} else {
		privateKey, err := wgtypes.ParseKey(privateValue)
		if err != nil {
			return err
		}
		publicValue = privateKey.PublicKey().String()
	}
	clientReality["public_key"] = publicValue

	shortID := strings.TrimSpace(stringValue(serverReality["short_id"]))
	if shortID == "" {
		shortID = defaultRealityShortID(tag, now)
		serverReality["short_id"] = shortID
	}
	clientReality["short_id"] = shortID
	return nil
}

func rawObject(raw json.RawMessage) (map[string]interface{}, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return map[string]interface{}{}, nil
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	if obj == nil {
		obj = map[string]interface{}{}
	}
	return obj, nil
}

func nestedObject(parent map[string]interface{}, key string) map[string]interface{} {
	if existing, ok := parent[key].(map[string]interface{}); ok {
		return existing
	}
	next := map[string]interface{}{}
	parent[key] = next
	return next
}

var domainInboundPartPattern = regexp.MustCompile(`[^A-Za-z0-9_.-]`)

func sanitizeDomainInboundPart(value string, fallback string) string {
	value = domainInboundPartPattern.ReplaceAllString(strings.TrimSpace(value), "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return fallback
	}
	return value
}

func buildClusterInboundTag(prefix string, baseTag string, nodeID string, suffix string) string {
	base := sanitizeDomainInboundPart(baseTag, "inbound")
	node := sanitizeDomainInboundPart(nodeID, "node")
	return prefix + base + "-" + node + suffix
}

func defaultRealityShortID(tag string, now int64) string {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%s-%d", tag, now)))
	if len(encoded) > 16 {
		return encoded[:16]
	}
	return encoded
}

func stringValue(value interface{}) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}
