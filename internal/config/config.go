package config

import (
	"flag"
	"fmt"
	"path/filepath"
)

const DefaultPort = 8573

type Config struct {
	Port         int
	DataDir      string
	LogDir       string
	DatabasePath string
}

func Load(args []string) (Config, error) {
	fs := flag.NewFlagSet("lightmonitor", flag.ContinueOnError)

	port := fs.Int("P", DefaultPort, "HTTP server port")
	dataDir := fs.String("data-dir", "data", "data directory")
	logDir := fs.String("log-dir", "log", "log directory")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	if *port <= 0 || *port > 65535 {
		return Config{}, fmt.Errorf("invalid port: %d", *port)
	}

	return Config{
		Port:         *port,
		DataDir:      *dataDir,
		LogDir:       *logDir,
		DatabasePath: filepath.Join(*dataDir, "lightmonitor.db"),
	}, nil
}
