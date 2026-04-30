package cronjob

import (
	"context"
	"sync"
	"time"

	service "github.com/BeanYa/b-ui/src/backend/internal/domain/services"
	"github.com/BeanYa/b-ui/src/backend/internal/domain/services/ping"
	logger "github.com/BeanYa/b-ui/src/backend/internal/infra/logging"
)

type ClusterMeshPingJob struct {
	mu      sync.Mutex
	lastRun map[uint]time.Time
}

func NewClusterMeshPingJob() *ClusterMeshPingJob {
	return &ClusterMeshPingJob{
		lastRun: make(map[uint]time.Time),
	}
}

func (j *ClusterMeshPingJob) Run() {
	cs := &service.ClusterService{}
	domains, err := cs.ListDomains()
	if err != nil {
		logger.Warning("ClusterMeshPingJob: list domains failed: ", err)
		return
	}

	members, err := cs.ListMembers()
	if err != nil {
		logger.Warning("ClusterMeshPingJob: list members failed: ", err)
		return
	}

	for _, domain := range domains {
		domainModel, err := cs.GetDomain(domain.ID)
		if err != nil {
			continue
		}
		policy := domainModel.GetPingPolicy()
		if !policy.Enabled {
			continue
		}

		// Check interval
		j.mu.Lock()
		last, ok := j.lastRun[domain.ID]
		j.mu.Unlock()
		if ok && time.Since(last) < time.Duration(policy.Interval)*time.Second {
			continue
		}

		j.runPing(domain.Domain, domain.ID, members, policy.MaxConcurrent)
	}
}

func (j *ClusterMeshPingJob) runPing(domainStr string, domainID uint, members []service.ClusterMemberResponse, maxConcurrent int) {
	if !ping.AcquireMeshPingLock() {
		return
	}
	defer ping.ReleaseMeshPingLock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pingMembers := make([]ping.MeshMember, 0)
	for _, m := range members {
		if m.DomainID != domainID {
			continue
		}
		conn, _ := (&service.ClusterService{}).GetMemberConnection(m.NodeID)
		pm := ping.MeshMember{
			MemberID: m.NodeID,
			NodeID:   m.NodeID,
			Name:     m.DisplayName,
			BaseURL:  m.BaseURL,
		}
		if m.Name != "" {
			pm.Name = m.Name
		}
		if conn != nil {
			pm.PeerToken = conn.Token
			pm.Address = ping.ExtractHostFromBaseURL(conn.BaseURL)
		}
		pingMembers = append(pingMembers, pm)
	}

	if len(pingMembers) <= 1 {
		return
	}

	identity := service.ClusterLocalIdentityService{}
	local, err := identity.GetOrCreate()
	if err != nil {
		logger.Warning("ClusterMeshPingJob: get local node ID failed: ", err)
		return
	}

	meshSvc := ping.NewMeshService()
	result, err := meshSvc.Run(ctx, domainStr, pingMembers, local.NodeID, maxConcurrent)
	if err != nil {
		logger.Warning("ClusterMeshPingJob: mesh ping failed for ", domainStr, ": ", err)
		return
	}

	ping.SetMeshResult(domainStr, result)

	store := ping.NewStore()
	if err := store.SaveMeshResult(result); err != nil {
		logger.Warning("ClusterMeshPingJob: save failed: ", err)
	}

	j.mu.Lock()
	j.lastRun[domainID] = time.Now()
	j.mu.Unlock()
}
