package output

import (
	"github.com/justalternate/fleettui/internal/domain"
)

type ConfigLoader interface {
	LoadConfig(path string) (*domain.Config, error)
	LoadHosts(path string) (*domain.HostsConfig, error)
}
