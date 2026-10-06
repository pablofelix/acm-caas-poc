package gpu

import (
	"log/slog"
	"sync"
	"time"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/config"
)

const DefaultNamespace = "open-cluster-management-policies"

type Manager struct {
	client     *client.Client
	cfg        config.Config
	logger     *slog.Logger
	mu         sync.Mutex
	reqs       map[string]*GPURequest
	reqTimeout time.Duration
}

func (m *Manager) requestTimeout() time.Duration {
	if m.reqTimeout > 0 {
		return m.reqTimeout
	}
	return 30 * time.Second
}

func New(c *client.Client, cfg config.Config, logger *slog.Logger) *Manager {
	return &Manager{client: c, cfg: cfg, logger: logger, reqs: make(map[string]*GPURequest)}
}
