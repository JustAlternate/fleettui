package output

import (
	"fleettui/internal/domain"
)

type ConfigLoader interface {
	LoadConfig(path string) (*domain.Config, error)
	LoadHosts(path string) (*domain.HostsConfig, error)
}
