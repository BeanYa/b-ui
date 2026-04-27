//go:build !with_naive_outbound

package core

import (
	logger "github.com/BeanYa/b-ui/src/backend/internal/infra/logging"
	"github.com/sagernet/sing-box/adapter/outbound"
)

func registerNaiveOutbound(registry *outbound.Registry) {
	// naive outbound is disabled when built without with_naive_outbound tag
	logger.Error("naive outbound is disabled when built without with_naive_outbound tag")
}
