package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/klcache/cache"
	"github.com/klcache/cluster"
	"github.com/klcache/config"
	"github.com/klcache/server"
)

func main() {
	cfg := config.LoadConfig()

	log.Printf("Starting KLCache node: %s", cfg.NodeName)

	// Initialize Storage
	store := cache.NewStore()

	// Initialize HashRing
	hashRing := cluster.NewHashRing()

	// Initialize Cluster Discovery (Memberlist)
	node, err := cluster.NewNode(cfg, hashRing)
	if err != nil {
		log.Fatalf("Failed to initialize cluster node: %v", err)
	}

	// Initialize and Start HTTP Server API
	apiServer := server.NewServer(cfg, store, hashRing)
	go func() {
		if err := apiServer.Start(); err != nil {
			log.Fatalf("API Server failed: %v", err)
		}
	}()

	// Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down node gracefully...")
	if node.MDNSServer != nil {
		node.MDNSServer.Shutdown()
	}
	if node.List != nil {
		node.List.Leave(time.Second * 5)
		node.List.Shutdown()
	}
}
