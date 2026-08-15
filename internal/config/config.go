package config

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Logging struct {
	Output string `yaml:"output"`
	File   string `yaml:"file"`
}
type Metrics struct {
	Enabled bool   `yaml:"enabled"`
	Listen  string `yaml:"listen"`
}
type Config struct {
	Interfaces    []string `yaml:"interface"`
	ReloadSeconds int      `yaml:"reload_interval"`
	LogLevel      string   `yaml:"log_level"`
	ReplyMAC      string   `yaml:"reply_mac"`
	Logging       Logging  `yaml:"logging"`
	Metrics       Metrics  `yaml:"metrics"`
}

func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil { return Config{}, err }
	var c Config
	decErr := yaml.Unmarshal(b, &c)
	if decErr != nil { return c, decErr }
	if c.ReloadSeconds == 0 { c.ReloadSeconds = 10 }
	if c.LogLevel == "" { c.LogLevel = "info" }
	if c.ReplyMAC == "" { c.ReplyMAC = "auto" }
	if c.Logging.Output == "" { c.Logging.Output = "stdout" }
	if c.Metrics.Listen == "" { c.Metrics.Listen = "127.0.0.1:9108" }
	return c, c.Validate()
}

func (c Config) Validate() error {
	if len(c.Interfaces) == 0 { return fmt.Errorf("at least one interface is required") }
	seen := make(map[string]struct{}, len(c.Interfaces))
	for _, n := range c.Interfaces {
		if strings.TrimSpace(n) == "" { return fmt.Errorf("interface name cannot be empty") }
		if _, ok := seen[n]; ok { return fmt.Errorf("duplicate interface %q", n) }
		seen[n] = struct{}{}
	}
	if c.ReloadSeconds < 1 { return fmt.Errorf("reload_interval must be at least 1 second") }
	switch strings.ToLower(c.LogLevel) { case "info", "warning", "warn", "error": default: return fmt.Errorf("invalid log_level %q", c.LogLevel) }
	if c.ReplyMAC != "auto" {
		m, err := net.ParseMAC(c.ReplyMAC); if err != nil || len(m) != 6 { return fmt.Errorf("reply_mac must be auto or a 6-byte MAC") }
	}
	switch c.Logging.Output { case "stdout", "journal", "file": default: return fmt.Errorf("logging.output must be stdout, journal, or file") }
	if c.Logging.Output == "file" && c.Logging.File == "" { return fmt.Errorf("logging.file is required for file output") }
	return nil
}
func (c Config) ReloadInterval() time.Duration { return time.Duration(c.ReloadSeconds) * time.Second }
