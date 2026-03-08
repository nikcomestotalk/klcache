package config

import (
	"log"
	"os"
	"strconv"
)

// Config holds the configuration for a KLCache sidecar node
type Config struct {
	BindAddr      string // The IP address for memberlist to bind to (use 0.0.0.0 in K8s)
	BindPort      int    // The port for memberlist gossip
	APIPort       int    // The port for the HTTP API (Proxy + Client API)
	NodeName      string // Unique identifier (in K8s: use HOSTNAME/pod name)
	JoinAddress   string // Explicit node to join (overrides discovery when set)
	AuthToken     string // Bearer token for inter-pod proxy traffic
	AppName       string // Application name — sidecars with same AppName form one cluster
	KubeNamespace string // Kubernetes namespace (for DNS discovery)
	PodIP         string // Pod IP in K8s (for advertising reachable API address)
}

func LoadConfig() Config {
	bindPort := getEnvAsInt("BIND_PORT", 8000)
	apiPort := getEnvAsInt("API_PORT", 9000)
	joinAddr := getEnv("JOIN_ADDR", "")
	authToken := getEnv("APP_AUTH_TOKEN", "")

	// App name: primary identifier for "same application" — all sidecars of this app cluster together
	appName := getEnv("APP_NAME", getEnv("APP_CLUSTER_ID", "default"))
	kubeNamespace := getEnv("KUBE_NAMESPACE", getEnv("POD_NAMESPACE", "default"))
	podIP := getEnv("POD_IP", "")

	// In Kubernetes: bind to all interfaces so other pods can reach us; use pod name and pod IP
	inK8s := os.Getenv("KUBERNETES_SERVICE_HOST") != ""
	var bindAddr, nodeName string
	if inK8s {
		bindAddr = getEnv("BIND_ADDR", "0.0.0.0")
		nodeName = getEnv("NODE_NAME", getEnv("HOSTNAME", "node-"+strconv.Itoa(apiPort)))
	} else {
		bindAddr = getEnv("BIND_ADDR", "127.0.0.1")
		nodeName = getEnv("NODE_NAME", "node-"+strconv.Itoa(apiPort))
	}

	return Config{
		BindAddr:      bindAddr,
		BindPort:      bindPort,
		APIPort:       apiPort,
		NodeName:      nodeName,
		JoinAddress:   joinAddr,
		AuthToken:     authToken,
		AppName:       appName,
		KubeNamespace: kubeNamespace,
		PodIP:         podIP,
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
