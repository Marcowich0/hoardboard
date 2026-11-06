// Package config reads hoardboard's settings from environment variables.
//
// Every setting has an environment variable, and nothing about the owner's
// folders is built in: the defaults are the paths inside the container, where
// the compose file mounts the host's folders.
package config

import (
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Marcowich0/hoardboard/internal/clock"
)

// Environment variables.
const (
	EnvMountRoot    = "HOARDBOARD_MOUNT_ROOT"
	EnvStatusDir    = "HOARDBOARD_STATUS_DIR"
	EnvDataDir      = "HOARDBOARD_DATA_DIR"
	EnvPort         = "HOARDBOARD_PORT"
	EnvScanInterval = "HOARDBOARD_SCAN_INTERVAL"
)

// Defaults, as seen from inside the container.
const (
	DefaultMountRoot    = "/mnt"
	DefaultStatusDir    = "/var/local/emhttp"
	DefaultDataDir      = "/data"
	DefaultPort         = 8098
	DefaultScanInterval = time.Hour
)

// Config is the server configuration.
type Config struct {
	// MountRoot holds one folder per Disk (disk1, disk2, …, cache), as /mnt
	// does on Unraid. Read-only.
	MountRoot string
	// StatusDir is Unraid's status folder, holding disks.ini and var.ini.
	// Read-only.
	StatusDir string
	// DataDir holds the SQLite database. Read-write.
	DataDir string
	// Port is the HTTP listen port.
	Port int
	// ScanInterval is the time between scheduled Scans.
	ScanInterval time.Duration

	// Revision is the commit the binary was built from, "" if unknown. No
	// variable sets it: cmd/hoardboard reads it from the build.
	Revision string
	// MountInfo is the file listing the mount points a Scan must not cross,
	// "" for /proc/self/mountinfo. No variable sets it: tests inject a fake.
	MountInfo string
	// Clock is the source of "now". No variable sets it: nil means the
	// system clock, and tests inject a fake.
	Clock clock.Clock
	// NoSchedule turns scheduled Scans off, so that Scans run only when
	// asked for. No variable sets it: tests turn it on, so that a server
	// doesn't scan on its own.
	NoSchedule bool
}

// FromEnv reads the configuration through getenv (os.Getenv in production).
func FromEnv(getenv func(string) string) (Config, error) {
	cfg := Config{
		MountRoot:    DefaultMountRoot,
		StatusDir:    DefaultStatusDir,
		DataDir:      DefaultDataDir,
		Port:         DefaultPort,
		ScanInterval: DefaultScanInterval,
	}
	for _, p := range []struct {
		env string
		dst *string
	}{
		{EnvMountRoot, &cfg.MountRoot},
		{EnvStatusDir, &cfg.StatusDir},
		{EnvDataDir, &cfg.DataDir},
	} {
		v := getenv(p.env)
		if v == "" {
			continue
		}
		if !filepath.IsAbs(v) {
			return Config{}, fmt.Errorf("%s=%q: want an absolute path", p.env, v)
		}
		*p.dst = filepath.Clean(v)
	}
	if v := getenv(EnvPort); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil || port < 1 || port > 65535 {
			return Config{}, fmt.Errorf("%s=%q: want a port number from 1 to 65535", EnvPort, v)
		}
		cfg.Port = port
	}
	if v := getenv(EnvScanInterval); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < time.Minute {
			return Config{}, fmt.Errorf("%s=%q: want a duration of at least 1m, such as 1h or 30m", EnvScanInterval, v)
		}
		cfg.ScanInterval = d
	}
	return cfg, nil
}
