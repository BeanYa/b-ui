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
	DB            *gorm.DB
	InboundSaver  clusterDomainInboundSaver
	Identity      clusterDomainInboundIdentity
	Broadcaster   clusterDomainInboundBroadcaster
	Reporter      clusterDomainInboundReporter
	PortAllocator func() (int, error)
	Now           func() int64
}

type ClusterDomainInboundService struct {
	db            *gorm.DB
	inboundSaver  clusterDomainInboundSaver
	identity      clusterDomainInboundIdentity
	broadcaster   clusterDomainInboundBroadcaster
	reporter      clusterDomainInboundReporter
	portAllocator func() (int, error)
	now           func() int64
}

type DomainInboundCreateResult struct {
	InboundID uint
	RequestID string
	Created   bool
}

func NewClusterDomainInboundService(opts ClusterDomainInboundServiceOptions) *ClusterDomainInboundService {
	s := &ClusterDomainInboundService{
		db:            opts.DB,
		inboundSaver:  opts.InboundSaver,
		identity:      opts.Identity,
		broadcaster:   opts.Broadcaster,
		reporter:      opts.Reporter,
		portAllocator: opts.PortAllocator,
		now:           opts.Now,
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
	if s.now == nil {
		s.now = func() int64 { return time.Now().Unix() }
	}
	return s
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

		inboundJSON, tag, err := s.prepareDomainInboundJSON(tx, payload, nodeID)
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

func (s *ClusterDomainInboundService) prepareDomainInboundJSON(tx *gorm.DB, payload clustertypes.DomainInboundCreatePayload, nodeID string) (json.RawMessage, string, error) {
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
	tag := payload.Prefix + baseTag + "-" + sanitizeDomainInboundPart(nodeID, "node") + payload.Suffix

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
	port, err := s.portAllocator()
	if err != nil {
		return nil, "", err
	}
	inbound["listen_port"] = port

	if payload.TLS != nil {
		tlsID, err := s.createDomainInboundTLS(tx, tag, payload)
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

func (s *ClusterDomainInboundService) createDomainInboundTLS(tx *gorm.DB, tag string, payload clustertypes.DomainInboundCreatePayload) (uint, error) {
	server, err := rawObject(payload.TLS.Server)
	if err != nil {
		return 0, err
	}
	client, err := rawObject(payload.TLS.Client)
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(payload.TLSTemplate) == "reality" {
		if err := ensureRealityKeypair(server, client, tag, s.now()); err != nil {
			return 0, err
		}
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
