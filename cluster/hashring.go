package cluster

import (
	"fmt"
	"log"
	"sync"

	"github.com/buraksezer/consistent"
	"github.com/cespare/xxhash/v2"
)

// In buraksezer/consistent, member needs to implement String() string
type ringMember string

func (m ringMember) String() string {
	return string(m)
}

// hasher implements consistent.Hasher interface using xxhash
type hasher struct{}

func (h hasher) Sum64(data []byte) uint64 {
	return xxhash.Sum64(data)
}

// HashRing manages the consistent hashing and mapping of keys to cluster nodes
type HashRing struct {
	mu          sync.RWMutex
	currRing    *consistent.Consistent
	memberAddrs map[string]string // Maps node name to API address (e.g. "node-9000" -> "127.0.0.1:9000")
}

// NewHashRing creates a new HashRing
func NewHashRing() *HashRing {
	cfg := consistent.Config{
		PartitionCount:    271,  // Good prime number for partitions
		ReplicationFactor: 20,   // Virtual nodes to improve distribution
		Load:              1.25, // Bounded load
		Hasher:            hasher{},
	}

	return &HashRing{
		currRing:    consistent.New(nil, cfg),
		memberAddrs: make(map[string]string),
	}
}

// AddMember adds a node to the hash ring
func (hr *HashRing) AddMember(nodeName string, apiAddress string) {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	hr.currRing.Add(ringMember(nodeName))
	hr.memberAddrs[nodeName] = apiAddress
	log.Printf("[HashRing] Added member %s (%s). Partitions dynamically re-assigned.\n", nodeName, apiAddress)
}

// RemoveMember removes a node from the hash ring
func (hr *HashRing) RemoveMember(nodeName string) {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	hr.currRing.Remove(nodeName)
	delete(hr.memberAddrs, nodeName)
	log.Printf("[HashRing] Removed member %s. Data will be re-assigned.\n", nodeName)
}

// GetOwner returns the node name and API address that owns the specified key
func (hr *HashRing) GetOwner(key string) (nodeName string, apiAddress string, err error) {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	owner := hr.currRing.LocateKey([]byte(key))
	if owner == nil {
		return "", "", fmt.Errorf("no members in hashring")
	}

	nodeName = owner.String()
	apiAddress = hr.memberAddrs[nodeName]
	return nodeName, apiAddress, nil
}
