package config

const (
	// Server configuration
	Protocol       = "tcp"
	Host           = "localhost"
	Port           = "6379"
	MaxConnections = 20000
	MaxEvents      = 1024

	// Eviction configuration
	MaxKeys         = 10000
	EvictionPolicy  = "random"
	EvictionSamples = 5
)
