package service

import (
	"context"

	clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
)

func (b *ClusterHTTPBroadcaster) BroadcastDomainCleanup(ctx context.Context, domain *model.ClusterDomain, payload clustertypes.DomainCleanupPayload) error {
	if b == nil || domain == nil {
		return nil
	}
	return b.broadcastDomainUserCommand(ctx, domain, PeerActionDomainCleanup, payload)
}
