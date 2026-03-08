package cluster

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/mdns"
	"github.com/hashicorp/memberlist"
	"github.com/klcache/config"
)

// Node represents the cluster state for a KLCache sidecar
type Node struct {
	List       *memberlist.Memberlist
	Cfg        config.Config
	HashRing   *HashRing
	MDNSServer *mdns.Server
}

// isKubernetes returns true when running inside a Kubernetes cluster
func isKubernetes() bool {
	return os.Getenv("KUBERNETES_SERVICE_HOST") != ""
}

// apiAddr returns the address other nodes use to reach this node's API (for proxying)
func apiAddr(cfg config.Config) string {
	// In K8s, BindAddr may be 0.0.0.0 — we must advertise a reachable IP so other pods can connect
	ip := cfg.PodIP
	if ip == "" && isKubernetes() {
		ip = inferPodIP(cfg)
	}
	if ip != "" {
		return fmt.Sprintf("%s:%d", ip, cfg.APIPort)
	}
	return fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.APIPort)
}

// inferPodIP tries to determine our reachable IP in K8s
func inferPodIP(cfg config.Config) string {
	// 1. Try hostname.app.namespace.svc.cluster.local (works for StatefulSet)
	dns := fmt.Sprintf("%s.%s.%s.svc.cluster.local", cfg.NodeName, cfg.AppName, cfg.KubeNamespace)
	if addrs, err := net.LookupHost(dns); err == nil && len(addrs) > 0 {
		return addrs[0]
	}
	// 2. Fallback: first non-loopback IPv4 from interfaces (works when bind to 0.0.0.0)
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

// NewNode initializes a new cluster node (sidecar instance)
func NewNode(cfg config.Config, ring *HashRing) (*Node, error) {
	node := &Node{
		Cfg:      cfg,
		HashRing: ring,
	}

	// Create a default memberlist config
	mConfig := memberlist.DefaultLocalConfig()
	mConfig.Name = cfg.NodeName
	mConfig.BindAddr = cfg.BindAddr
	mConfig.BindPort = cfg.BindPort

	// Disabling default logging so it doesn't clutter output, unless debugging
	mConfig.Logger = log.New(os.Stderr, "[Memberlist] ", log.LstdFlags)

	// Custom Event Delegate to listen to join/leave events
	mConfig.Events = &eventDelegate{node: node}
	// Custom Delegate to share API port
	mConfig.Delegate = &nodeDelegate{apiPort: fmt.Sprintf("%d", cfg.APIPort)}

	list, err := memberlist.Create(mConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create memberlist: %w", err)
	}
	node.List = list

	// If explicit join address is set, use it (overrides discovery)
	if cfg.JoinAddress != "" {
		_, err := list.Join([]string{cfg.JoinAddress})
		if err != nil {
			log.Printf("Failed to join cluster at %s: %v\n", cfg.JoinAddress, err)
		} else {
			log.Printf("Successfully joined cluster at %s\n", cfg.JoinAddress)
		}
	}

	// Add self to the hash ring — use PodIP in K8s so other pods can reach our API
	selfAddr := apiAddr(cfg)
	if isKubernetes() && strings.HasPrefix(selfAddr, "0.0.0.0:") {
		log.Printf("Warning: Advertising 0.0.0.0 — other pods cannot reach this sidecar. Set POD_IP via downward API (status.podIP).")
	}
	node.HashRing.AddMember(cfg.NodeName, selfAddr)

	// Start mDNS broadcast (local/dev only — multicast often disabled in K8s)
	if !isKubernetes() {
		info := []string{"KLCache Sidecar"}
		serviceType := fmt.Sprintf("_klcache_%s._tcp", cfg.AppName)
		mService, err := mdns.NewMDNSService(cfg.NodeName, serviceType, "", "", cfg.BindPort, nil, info)
		if err != nil {
			log.Printf("Warning: Failed to create mDNS service: %v", err)
		} else {
			mServer, err := mdns.NewServer(&mdns.Config{Zone: mService})
			if err != nil {
				log.Printf("Warning: Failed to start mDNS server: %v", err)
			} else {
				node.MDNSServer = mServer
				log.Printf("mDNS broadcasting for app=%s on %s:%d", cfg.AppName, serviceType, cfg.BindPort)
			}
		}
	}

	// Discover other sidecars of the same application
	go discoverPeers(node)

	return node, nil
}

// discoverPeers finds other KLCache sidecars for the same application and joins them
func discoverPeers(node *Node) {
	if isKubernetes() {
		discoverPeersK8s(node)
		return
	}
	discoverPeersMDNS(node)
}

// discoverPeersK8s uses Kubernetes DNS (headless service) to find other sidecars of the same app
func discoverPeersK8s(node *Node) {
	cfg := node.Cfg
	dnsName := fmt.Sprintf("%s.%s.svc.cluster.local", cfg.AppName, cfg.KubeNamespace)
	log.Printf("[Sidecar] Discovering peers for app=%s via DNS: %s", cfg.AppName, dnsName)

	for {
		addrs, err := net.LookupHost(dnsName)
		if err != nil {
			log.Printf("[Sidecar] DNS lookup %s failed: %v (will retry)", dnsName, err)
			time.Sleep(10 * time.Second)
			continue
		}

		// Filter out our own pod IP so we don't try to join ourselves
		myIP := cfg.PodIP
		if myIP == "" {
			myIP = inferPodIP(cfg)
		}

		for _, ip := range addrs {
			if ip == myIP {
				continue
			}
			gossipAddr := fmt.Sprintf("%s:%d", ip, cfg.BindPort)
			if _, err := node.List.Join([]string{gossipAddr}); err != nil {
				// Already joined or unreachable — memberlist deduplicates
				_ = err
			}
		}

		time.Sleep(10 * time.Second)
	}
}

// discoverPeersMDNS uses mDNS to find other sidecars on the local network (dev/local)
func discoverPeersMDNS(node *Node) {
	for {
		entriesCh := make(chan *mdns.ServiceEntry, 10)
		go func() {
			for entry := range entriesCh {
				if !strings.Contains(entry.Name, node.Cfg.NodeName) {
					addr := fmt.Sprintf("%v:%d", entry.AddrV4, entry.Port)
					_, _ = node.List.Join([]string{addr})
				}
			}
		}()

		serviceType := fmt.Sprintf("_klcache_%s._tcp", node.Cfg.AppName)
		_ = mdns.Query(&mdns.QueryParam{
			Service: serviceType,
			Domain:  "local",
			Timeout: 3 * time.Second,
			Entries: entriesCh,
		})
		close(entriesCh)

		time.Sleep(10 * time.Second)
	}
}

// nodeDelegate is used to broadcast local node information
type nodeDelegate struct {
	apiPort string
}

func (d *nodeDelegate) NodeMeta(limit int) []byte {
	return []byte(d.apiPort)
}

func (d *nodeDelegate) NotifyMsg(b []byte)                         {}
func (d *nodeDelegate) GetBroadcasts(overhead, limit int) [][]byte { return nil }
func (d *nodeDelegate) LocalState(join bool) []byte                { return nil }
func (d *nodeDelegate) MergeRemoteState(buf []byte, join bool)     {}

// eventDelegate handles cluster membership changes
type eventDelegate struct {
	node *Node
}

func (e *eventDelegate) NotifyJoin(meta *memberlist.Node) {
	log.Printf("Node joined: %s (%s)\n", meta.Name, meta.Addr)
	apiPort := string(meta.Meta)
	apiAddr := meta.Name // fallback
	if apiPort != "" {
		apiAddr = fmt.Sprintf("%s:%s", meta.Addr.String(), apiPort)
	}
	e.node.HashRing.AddMember(meta.Name, apiAddr)
}

func (e *eventDelegate) NotifyLeave(meta *memberlist.Node) {
	log.Printf("Node left: %s\n", meta.Name)
	e.node.HashRing.RemoveMember(meta.Name)
}

func (e *eventDelegate) NotifyUpdate(meta *memberlist.Node) {
	apiPort := string(meta.Meta)
	apiAddr := meta.Name
	if apiPort != "" {
		apiAddr = fmt.Sprintf("%s:%s", meta.Addr.String(), apiPort)
	}
	// We could re-add it to update the address
	e.node.HashRing.AddMember(meta.Name, apiAddr)
}
