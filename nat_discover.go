package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"sync"

	"github.com/jiangz222/go-nat-discovery/nats"
	"github.com/pion/logging"
)

type Config struct {
	MaxProcs      int    `json:"max_procs"`
	PrimaryAddr   string `json:"primaryAddr"`
	SecondaryAddr string `json:"secondaryAddr"`
	Pri2SecAddr   string `json:"pri2SecAddr"`
	Role          string `json:"role"`
	DebugLevel    int    `json:"debug_level"`
}

var (
	cfgPath = flag.String("f", "nat_discovery.conf", "config file path")
)

func init() {
	flag.Parse()
}

func parseConfig() *Config {
	data, err := os.ReadFile(*cfgPath)
	if err != nil {
		fmt.Println("read config file failed:", err)
		return nil
	}

	cfg := &Config{}
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		fmt.Println("unmarshal config failed:", err)
		return nil
	}

	if cfg.Role != "both" && cfg.Role != "pri" && cfg.Role != "sec" {
		fmt.Println("wrong role, only support: both 、 pri 、 sec")
		return nil
	}

	if cfg.Role == "pri" && cfg.Pri2SecAddr == "" {
		fmt.Println("need pri2SecAddr when current role is pri")
		return nil
	}

	if cfg.PrimaryAddr == "" || cfg.SecondaryAddr == "" {
		fmt.Println("need primary addr and secondary addr")
		return nil
	}

	return cfg
}

func main() {
	cfg := parseConfig()
	if cfg == nil {
		return
	}

	runtime.GOMAXPROCS(cfg.MaxProcs)

	level := logging.LogLevelInfo
	switch cfg.DebugLevel {
	case 1:
		level = logging.LogLevelDebug
	case 2:
		level = logging.LogLevelTrace
	}

	s, err := nats.NewSTUNServer(&nats.STUNServerConfig{
		PrimaryAddress:   cfg.PrimaryAddr,
		SecondaryAddress: cfg.SecondaryAddr,
		Role:             cfg.Role,
		Pri2SecHost:      cfg.Pri2SecAddr,
		LogLevel:         level,
	})
	if err != nil {
		fmt.Println("err new stun server")
		return
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	s.Start()
	if cfg.Role == "sec" {
		wg.Done()
		s.StartListenServer()
	}
	wg.Wait()
}
