package config

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"slices"
	"strings"
)

func gunzip(data []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return io.ReadAll(gz)
}

type ProcessConfig struct {
	HostedServices    []string       `json:"hosted_services"`
	HostedGateways    []string       `json:"hosted_gateways"`
	LocalServicePorts map[string]int `json:"local_service_ports"`
}

// ParseRuntime parses the Encore runtime config.
func ParseRuntime(config, processCfg, deployID string) *Runtime {
	if config == "" {
		log.Fatalln("encore runtime: fatal error: no encore runtime config provided")
	}

	// We used to support RawURLEncoding, but now we use StdEncoding.
	// Try both if StdEncoding fails.
	var (
		bytes []byte
		err   error
	)
	config, isGzipped := strings.CutPrefix(config, "gzip:")
	// nosemgrep
	if bytes, err = base64.StdEncoding.DecodeString(config); err != nil {
		bytes, err = base64.RawURLEncoding.DecodeString(config)
	}
	if err != nil {
		log.Fatalln("encore runtime: fatal error: could not decode encore runtime config:", err)
	}
	if isGzipped {
		if bytes, err = gunzip(bytes); err != nil {
			log.Fatalln("encore runtime: fatal error: could not gunzip encore runtime config:", err)
		}
	}
	var cfg Runtime
	if err := json.Unmarshal(bytes, &cfg); err != nil {
		log.Fatalln("encore runtime: fatal error: could not parse encore runtime config:", err)
	}

	if _, err := url.Parse(cfg.APIBaseURL); err != nil {
		log.Fatalln("encore runtime: fatal error: could not parse api base url from encore runtime config:", err)
	}

	if processCfg != "" {
		if bytes, err = base64.StdEncoding.DecodeString(processCfg); err != nil {
			log.Fatalln("encore runtime: fatal error: could not decode encore process config:", err)
		}
		var procCfg ProcessConfig
		if err := json.Unmarshal(bytes, &procCfg); err != nil {
			log.Fatalln("encore runtime: fatal error: could not parse encore process config:", err)
		}
		cfg.HostedServices = procCfg.HostedServices
		var hostedGateways []Gateway
		for _, name := range procCfg.HostedGateways {
			i := slices.IndexFunc(cfg.Gateways, func(gw Gateway) bool { return gw.Name == name })
			if i == -1 {
				log.Fatalf("encore runtime: fatal error: gateway %q not found in runtime config", name)
			}
			hostedGateways = append(hostedGateways, cfg.Gateways[i])
		}
		cfg.Gateways = hostedGateways

		// Use noop service auth method if not specified
		svcAuth := ServiceAuth{"noop"}
		if len(cfg.ServiceAuth) > 0 {
			// Use the first service auth method from the runtime config
			svcAuth = cfg.ServiceAuth[0]
		}

		for name, port := range procCfg.LocalServicePorts {
			if cfg.ServiceDiscovery == nil {
				cfg.ServiceDiscovery = make(map[string]Service)
			}
			cfg.ServiceDiscovery[name] = Service{
				Name:        name,
				URL:         fmt.Sprintf("http://localhost:%d", port),
				Protocol:    Http,
				ServiceAuth: svcAuth,
			}
		}
	}

	// If the environment deploy ID is set, use that instead of the one
	// embedded in the runtime config
	if deployID != "" {
		cfg.DeployID = deployID
	}

	return &cfg
}

// ParseStatic parses the Encore static config.
func ParseStatic(config string) *Static {
	if config == "" {
		log.Fatalln("encore runtime: fatal error: no encore static config provided")
	}
	bytes, err := base64.StdEncoding.DecodeString(config)
	if err != nil {
		log.Fatalln("encore runtime: fatal error: could not decode encore static config:", err)
	}
	var cfg Static
	if err := json.Unmarshal(bytes, &cfg); err != nil {
		log.Fatalln("encore runtime: fatal error: could not parse encore static config:", err)
	}
	return &cfg
}
