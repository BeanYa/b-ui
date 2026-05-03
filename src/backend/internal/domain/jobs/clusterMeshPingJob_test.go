package cronjob

import (
	"testing"

	service "github.com/BeanYa/b-ui/src/backend/internal/domain/services"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
)

func TestClusterMeshPingJobSkipsMembersWithoutDomains(t *testing.T) {
	stub := &stubClusterMeshPingService{}
	original := newClusterServiceForMeshPing
	newClusterServiceForMeshPing = func() clusterMeshPingService {
		return stub
	}
	t.Cleanup(func() {
		newClusterServiceForMeshPing = original
	})

	NewClusterMeshPingJob().Run()

	if stub.listMembersCalls != 0 {
		t.Fatalf("expected no member lookup without domains, got %d calls", stub.listMembersCalls)
	}
}

type stubClusterMeshPingService struct {
	domains          []service.ClusterDomainResponse
	members          []service.ClusterMemberResponse
	listMembersCalls int
}

func (s *stubClusterMeshPingService) ListDomains() ([]service.ClusterDomainResponse, error) {
	return s.domains, nil
}

func (s *stubClusterMeshPingService) ListMembers() ([]service.ClusterMemberResponse, error) {
	s.listMembersCalls++
	return s.members, nil
}

func (s *stubClusterMeshPingService) GetDomain(id uint) (*model.ClusterDomain, error) {
	return &model.ClusterDomain{Id: id}, nil
}

func (s *stubClusterMeshPingService) GetMemberConnection(string) (*service.ClusterMemberConnectionResponse, error) {
	return nil, nil
}
