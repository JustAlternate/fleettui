package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/justalternate/fleettui/internal/domain"
)

func TestLoader_LoadConfig_ValidFile(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantRefresh time.Duration
		wantMetrics []domain.MetricType
		wantErr     bool
	}{
		{
			name: "valid config with all fields",
			content: `refresh_rate: 10s
metrics:
  - cpu
  - ram
  - network`,
			wantRefresh: 10 * time.Second,
			wantMetrics: []domain.MetricType{"cpu", "ram", "network"},
			wantErr:     false,
		},
		{
			name: "valid config with default refresh rate",
			content: `metrics:
  - cpu`,
			wantRefresh: 5 * time.Second,
			wantMetrics: []domain.MetricType{"cpu"},
			wantErr:     false,
		},
		{
			name:        "empty metrics uses defaults",
			content:     `refresh_rate: 5s`,
			wantRefresh: 5 * time.Second,
			wantMetrics: []domain.MetricType{
				domain.MetricCPU,
				domain.MetricRAM,
				domain.MetricNetwork,
				domain.MetricConnectivity,
				domain.MetricUptime,
				domain.MetricSystemd,
				domain.MetricOS,
			},
			wantErr: false,
		},
		{
			name: "invalid duration format",
			content: `refresh_rate: invalid
metrics:
  - cpu`,
			wantRefresh: 0,
			wantMetrics: nil,
			wantErr:     true,
		},
		{
			name: "invalid YAML",
			content: `refresh_rate: 5s
metrics:
  - cpu
  ram`,
			wantRefresh: 0,
			wantMetrics: nil,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")

			if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			loader := NewLoader()
			config, err := loader.LoadConfig(configPath)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if config.RefreshRate != tt.wantRefresh {
				t.Errorf("RefreshRate = %v, want %v", config.RefreshRate, tt.wantRefresh)
			}

			if len(config.EnabledMetrics) != len(tt.wantMetrics) {
				t.Errorf("len(EnabledMetrics) = %v, want %v", len(config.EnabledMetrics), len(tt.wantMetrics))
			}

			for i, metric := range config.EnabledMetrics {
				if i < len(tt.wantMetrics) && metric != tt.wantMetrics[i] {
					t.Errorf("EnabledMetrics[%d] = %v, want %v", i, metric, tt.wantMetrics[i])
				}
			}
		})
	}
}

func TestLoader_LoadConfig_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent.yaml")

	loader := NewLoader()
	config, err := loader.LoadConfig(configPath)

	if err != nil {
		t.Errorf("expected no error for missing file, got: %v", err)
	}

	if config == nil {
		t.Fatal("expected default config, got nil")
	}

	if config.RefreshRate != 5*time.Second {
		t.Errorf("expected default refresh rate 5s, got: %v", config.RefreshRate)
	}

	expectedMetrics := []domain.MetricType{
		domain.MetricCPU,
		domain.MetricRAM,
		domain.MetricNetwork,
		domain.MetricConnectivity,
		domain.MetricUptime,
		domain.MetricSystemd,
		domain.MetricOS,
	}

	if len(config.EnabledMetrics) != len(expectedMetrics) {
		t.Errorf("expected %d default metrics, got %d", len(expectedMetrics), len(config.EnabledMetrics))
	}
}

func TestLoader_LoadHosts_ValidFile(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantHosts int
		wantErr   bool
	}{
		{
			name: "valid hosts file with multiple hosts",
			content: `hosts:
  - name: server-01
    ip: 192.168.1.10
    user: root
    ssh_key_path: /home/user/.ssh/id_rsa
  - name: server-02
    ip: 192.168.1.11
    user: admin
    ssh_key_path: /home/user/.ssh/id_ed25519`,
			wantHosts: 2,
			wantErr:   false,
		},
		{
			name: "valid hosts file with minimal fields",
			content: `hosts:
  - name: server-01
    ip: 192.168.1.10`,
			wantHosts: 1,
			wantErr:   false,
		},
		{
			name:      "empty hosts list",
			content:   `hosts: []`,
			wantHosts: 0,
			wantErr:   false,
		},
		{
			name: "invalid YAML",
			content: `hosts:
  - name: server-01
    ip: 192.168.1.10
  invalid yaml here`,
			wantHosts: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			hostsPath := filepath.Join(tmpDir, "hosts.yaml")

			if err := os.WriteFile(hostsPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			loader := NewLoader()
			hosts, err := loader.LoadHosts(hostsPath)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(hosts.Hosts) != tt.wantHosts {
				t.Errorf("len(Hosts) = %v, want %v", len(hosts.Hosts), tt.wantHosts)
			}
		})
	}
}

func TestLoader_LoadHosts_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	hostsPath := filepath.Join(tmpDir, "nonexistent.yaml")

	loader := NewLoader()
	hosts, err := loader.LoadHosts(hostsPath)

	if err != nil {
		t.Errorf("expected no error for missing file, got: %v", err)
	}

	if hosts == nil {
		t.Fatal("expected empty hosts config, got nil")
	}

	if len(hosts.Hosts) != 0 {
		t.Errorf("expected empty hosts list, got %d hosts", len(hosts.Hosts))
	}
}

func TestLoader_LoadHosts_DefaultValues(t *testing.T) {
	tmpDir := t.TempDir()
	hostsPath := filepath.Join(tmpDir, "hosts.yaml")

	content := `hosts:
  - name: server-01
    ip: 192.168.1.10
  - name: server-02
    ip: 192.168.1.11
    user: admin`

	if err := os.WriteFile(hostsPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	loader := NewLoader()
	hosts, err := loader.LoadHosts(hostsPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(hosts.Hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(hosts.Hosts))
	}

	if hosts.Hosts[0].User != "root" {
		t.Errorf("expected default user 'root' for host 0, got: %v", hosts.Hosts[0].User)
	}

	if hosts.Hosts[1].User != "admin" {
		t.Errorf("expected user 'admin' for host 1, got: %v", hosts.Hosts[1].User)
	}
}

func TestLoader_findFirstSSHKey(t *testing.T) {
	tests := []struct {
		name    string
		files   []string
		wantKey string
	}{
		{
			name:    "finds id_rsa",
			files:   []string{"id_rsa", "config", "known_hosts"},
			wantKey: "id_rsa",
		},
		{
			name:    "finds id_ed25519 when id_rsa missing",
			files:   []string{"id_ed25519", "config"},
			wantKey: "id_ed25519",
		},
		{
			name:    "skips config file",
			files:   []string{"config", "id_rsa"},
			wantKey: "id_rsa",
		},
		{
			name:    "skips known_hosts file",
			files:   []string{"known_hosts", "id_rsa"},
			wantKey: "id_rsa",
		},
		{
			name:    "skips authorized_keys file",
			files:   []string{"authorized_keys", "id_rsa"},
			wantKey: "id_rsa",
		},
		{
			name:    "skips .pub files",
			files:   []string{"id_rsa.pub", "id_rsa"},
			wantKey: "id_rsa",
		},
		{
			name:    "returns empty when no valid keys",
			files:   []string{"config", "known_hosts", "authorized_keys", "id_rsa.pub"},
			wantKey: "",
		},
		{
			name:    "returns empty for empty directory",
			files:   []string{},
			wantKey: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			for _, file := range tt.files {
				path := filepath.Join(tmpDir, file)
				if err := os.WriteFile(path, []byte("test key"), 0600); err != nil {
					t.Fatalf("failed to create test file: %v", err)
				}
			}

			loader := NewLoader()
			got := loader.findFirstSSHKey(tmpDir)

			expectedPath := ""
			if tt.wantKey != "" {
				expectedPath = filepath.Join(tmpDir, tt.wantKey)
			}

			if got != expectedPath {
				t.Errorf("findFirstSSHKey() = %v, want %v", got, expectedPath)
			}
		})
	}
}

func TestLoader_findFirstSSHKey_MissingDir(t *testing.T) {
	loader := NewLoader()
	got := loader.findFirstSSHKey("/nonexistent/directory")

	if got != "" {
		t.Errorf("expected empty string for missing directory, got: %v", got)
	}
}

func TestLoader_SaveHosts(t *testing.T) {
	tests := []struct {
		name    string
		hosts   *domain.HostsConfig
		wantErr bool
	}{
		{
			name: "save single host",
			hosts: &domain.HostsConfig{
				Hosts: []domain.HostConfig{
					{Name: "server-01", IP: "192.168.1.10", User: "root"},
				},
			},
			wantErr: false,
		},
		{
			name: "save multiple hosts",
			hosts: &domain.HostsConfig{
				Hosts: []domain.HostConfig{
					{Name: "server-01", IP: "192.168.1.10", User: "root"},
					{Name: "server-02", IP: "192.168.1.11", User: "admin"},
				},
			},
			wantErr: false,
		},
		{
			name:    "save empty hosts",
			hosts:   &domain.HostsConfig{Hosts: []domain.HostConfig{}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			hostsPath := filepath.Join(tmpDir, "hosts.yaml")

			loader := NewLoader()
			err := loader.SaveHosts(hostsPath, tt.hosts)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			data, err := os.ReadFile(hostsPath)
			if err != nil {
				t.Fatalf("failed to read saved file: %v", err)
			}

			if len(data) == 0 {
				t.Error("saved file is empty")
			}

			loaded, err := loader.LoadHosts(hostsPath)
			if err != nil {
				t.Fatalf("failed to load saved file: %v", err)
			}

			if len(loaded.Hosts) != len(tt.hosts.Hosts) {
				t.Errorf("loaded %d hosts, expected %d", len(loaded.Hosts), len(tt.hosts.Hosts))
			}
		})
	}
}

func TestLoader_SaveConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *domain.Config
		wantErr bool
	}{
		{
			name: "save config with metrics",
			config: &domain.Config{
				RefreshRate: 10 * time.Second,
				EnabledMetrics: []domain.MetricType{
					domain.MetricCPU,
					domain.MetricRAM,
				},
			},
			wantErr: false,
		},
		{
			name: "save config with all metrics",
			config: &domain.Config{
				RefreshRate:    5 * time.Second,
				EnabledMetrics: []domain.MetricType{domain.MetricCPU, domain.MetricRAM, domain.MetricNetwork},
			},
			wantErr: false,
		},
		{
			name: "save empty metrics",
			config: &domain.Config{
				RefreshRate:    5 * time.Second,
				EnabledMetrics: []domain.MetricType{},
			},
			// Note: LoadConfig applies default metrics when list is empty
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")

			loader := NewLoader()
			err := loader.SaveConfig(configPath, tt.config)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			data, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatalf("failed to read saved file: %v", err)
			}

			if len(data) == 0 {
				t.Error("saved file is empty")
			}

			loaded, err := loader.LoadConfig(configPath)
			if err != nil {
				t.Fatalf("failed to load saved file: %v", err)
			}

			// Note: LoadConfig applies default metrics when list is empty
			expectedCount := len(tt.config.EnabledMetrics)
			if expectedCount == 0 {
				expectedCount = 7 // default metrics
			}
			if len(loaded.EnabledMetrics) != expectedCount {
				t.Errorf("loaded %d metrics, expected %d", len(loaded.EnabledMetrics), expectedCount)
			}
		})
	}
}

func TestLoader_SaveHosts_InvalidPath(t *testing.T) {
	loader := NewLoader()
	hosts := &domain.HostsConfig{
		Hosts: []domain.HostConfig{{Name: "test", IP: "192.168.1.1"}},
	}

	err := loader.SaveHosts("/nonexistent/directory/hosts.yaml", hosts)
	if err == nil {
		t.Error("expected error for invalid path, got nil")
	}
}
