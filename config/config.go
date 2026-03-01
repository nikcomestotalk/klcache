package config

import (
	"log"
	"os"
	"strconv"
)

// Config holds the configuration for a KLCache node
type Config struct {
	BindAddr     string // The IP address for memberlist to bind to
	BindPort     int    // The port for memberlist to bind to
	APIPort      int    // The port for the HTTP API (Proxy + Client API)
	NodeName     string // Unique name for this node
	JoinAddress  string // Address of an existing node to join (empty if first node)
	AuthToken    string // Optional token string to restrict API access
	AppClusterID string // Identifies the cluster to join via mDNS (defaults to "default")
}

func LoadConfig() Config {
	bindAddr := getEnv("BIND_ADDR", "127.0.0.1")
	bindPort := getEnvAsInt("BIND_PORT", 8000)
	apiPort := getEnvAsInt("API_PORT", 9000)
	nodeName := getEnv("NODE_NAME", "node-"+strconv.Itoa(apiPort))
	joinAddr := getEnv("JOIN_ADDR", "")
	authToken := getEnv("APP_AUTH_TOKEN", "")
	appClusterID := getEnv("APP_CLUSTER_ID", "default")

	return Config{
		BindAddr:     bindAddr,
		BindPort:     bindPort,
		APIPort:      apiPort,
		NodeName:     nodeName,
		JoinAddress:  joinAddr,
		AuthToken:    authToken,
		AppClusterID: appClusterID,
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	if valStr, ok := os.LookupEnv(key); ok {
		if val, err := strconv.Atoi(valStr); err == nil {
			return val
		} else {
			log.Printf("Warning: Invalid value for %s, falling back to %d\n", key, defaultVal)
		}
	}
	return defaultVal
}
