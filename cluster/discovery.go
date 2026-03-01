package cluster

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/mdns"
	"github.com/hashicorp/memberlist"
	"github.com/klcache/config"
)

// Node represents the cluster state
type Node struct {
	List       *memberlist.Memberlist
	Cfg        config.Config
	HashRing   *HashRing
	MDNSServer *mdns.Server
}

// NewNode initializes a new cluster node
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

	// If there's a node to join, join it
	if cfg.JoinAddress != "" {
		_, err := list.Join([]string{cfg.JoinAddress})
		if err != nil {
			log.Printf("Failed to join cluster at %s: %v\n", cfg.JoinAddress, err)
		} else {
			log.Printf("Successfully joined cluster at %s\n", cfg.JoinAddress)
		}
	}

	// Add self to the hash ring
	node.HashRing.AddMember(cfg.NodeName, fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.APIPort))

	// Start mDNS broadcast
	info := []string{"KLCache Node"}
	serviceType := fmt.Sprintf("_klcache_%s._tcp", cfg.AppClusterID)
	mService, err := mdns.NewMDNSService(cfg.NodeName, serviceType, "", "", cfg.BindPort, nil, info)
	if err != nil {
		log.Printf("Warning: Failed to create mDNS service: %v", err)
	} else {
		mServer, err := mdns.NewServer(&mdns.Config{Zone: mService})
		if err != nil {
			log.Printf("Warning: Failed to start mDNS server: %v", err)
		} else {
			node.MDNSServer = mServer
			log.Printf("mDNS broadcasting on %s:%d", serviceType, cfg.BindPort)
		}
	}

	// Discover other nodes via mDNS continuously in the background
	go discoverPeers(node)

	return node, nil
}

// discoverPeers routinely queries mDNS to find other klcache nodes and joins them automatically
func discoverPeers(node *Node) {
	for {
		entriesCh := make(chan *mdns.ServiceEntry, 10)
		go func() {
			for entry := range entriesCh {
				// Don't log or attempt to join ourselves
				if !strings.Contains(entry.Name, node.Cfg.NodeName) {
					addr := fmt.Sprintf("%v:%d", entry.AddrV4, entry.Port)
					// Only join if we haven't seen them yet, though memberlist takes care of deduplication silently
					_, _ = node.List.Join([]string{addr})
				}
			}
		}()

		serviceType := fmt.Sprintf("_klcache_%s._tcp", node.Cfg.AppClusterID)
		_ = mdns.Query(&mdns.QueryParam{
			Service: serviceType,
			Domain:  "local",
			Timeout: 3 * time.Second,
			Entries: entriesCh,
		})
		close(entriesCh)

		// Wait before querying again
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
