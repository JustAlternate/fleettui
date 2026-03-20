package output

import (
	"github.com/justalternate/fleetui/internal/domain"
)

type ConfigLoader interface {
	LoadConfig(path string) (*domain.Config, error)
	LoadHosts(path string) (*domain.HostsConfig, error)
}
