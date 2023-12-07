package main

// used for config file parsing

type ProbeConfig struct {
	CheckInterval uint16 `yaml:"check_interval"`
	Timeout       uint8  `yaml:"timeout"`
	Count         uint8  `yaml:"success_count"`
}

type HealthCheckConfig struct {
	Protocol       string      `yaml:"protocol"`
	Port           uint16      `yaml:"port"`
	StartAvailable bool        `yaml:"start_available"`
	Probe          ProbeConfig `yaml:"probe"`
}

type UpstreamDnsConfig struct {
	Servers []string `yaml:"servers"`
	Ttl     uint32   `yaml:"ttl"`
}

type UpstreamsConfig struct {
	Name        string            `yaml:"name"`
	Host        string            `yaml:"host"`
	Port        uint16            `yaml:"port"`
	Dns         UpstreamDnsConfig `yaml:"dns"`
	HealthCheck HealthCheckConfig `yaml:"health_check"`
}

type UpstreamGroupConfig struct {
	Name         string            `yaml:"name"`
	Distribution string            `yaml:"distribution"`
	Upstreams    []UpstreamsConfig `yaml:"upstreams"`
}

type TargetsConfig struct {
	Name          string              `yaml:"name"`
	Protocol      string              `yaml:"protocol"`
	Ip            string              `yaml:"ip"`
	Port          uint16              `yaml:"port"`
	UpstreamGroup UpstreamGroupConfig `yaml:"upstream_group"`
}

type LbConfig struct {
	Engine        string          `yaml:"engine"`
	TargetsConfig []TargetsConfig `yaml:"targets"`
}

type ConfigYaml struct {
	LbConfig []LbConfig `yaml:"lb"`
}
