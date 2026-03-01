package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/klcache/cache"
	"github.com/klcache/cluster"
	"github.com/klcache/config"
)

type Server struct {
	Cfg      config.Config
	Store    *cache.Store
	HashRing *cluster.HashRing
	Client   *http.Client
}

func NewServer(cfg config.Config, store *cache.Store, hashRing *cluster.HashRing) *Server {
	return &Server{
		Cfg:      cfg,
		Store:    store,
		HashRing: hashRing,
		Client:   &http.Client{},
	}
}

func (s *Server) Start() error {
	http.HandleFunc("/get", AuthMiddleware(s.Cfg.AuthToken, s.handleGet))
	http.HandleFunc("/set", AuthMiddleware(s.Cfg.AuthToken, s.handleSet))
	http.HandleFunc("/delete", AuthMiddleware(s.Cfg.AuthToken, s.handleDelete))
	http.HandleFunc("/keys", AuthMiddleware(s.Cfg.AuthToken, s.handleKeys))

	addr := fmt.Sprintf(":%d", s.Cfg.APIPort)
	log.Printf("Starting HTTP API server on %s\n", addr)
	return http.ListenAndServe(addr, nil)
}

func (s *Server) handleSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Missing 'key' query parameter", http.StatusBadRequest)
		return
	}

	forceLocal := r.URL.Query().Get("local") == "true"

	if !forceLocal {
		ownerNode, ownerAddr, err := s.HashRing.GetOwner(key)
		if err != nil {
			http.Error(w, "Cluster error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if ownerNode != s.Cfg.NodeName {
			log.Printf("Proxying SET %s to %s\n", key, ownerNode)
			s.proxyRequest(w, r, ownerAddr)
			return
		}
	} else {
		log.Printf("Bypassing cluster for SET %s due to local=true\n", key)
	}

	var value interface{}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		// Just store as string if we can't read it? No, expect JSON body for simple types
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	err = json.Unmarshal(body, &value)
	if err != nil {
		// treat raw body as string
		value = string(body)
	}

	err = s.Store.Set(key, value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Missing 'key' query parameter", http.StatusBadRequest)
		return
	}

	forceLocal := r.URL.Query().Get("local") == "true"

	if !forceLocal {
		ownerNode, ownerAddr, err := s.HashRing.GetOwner(key)
		if err != nil {
			http.Error(w, "Cluster error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if ownerNode != s.Cfg.NodeName {
			log.Printf("Proxying GET %s to %s\n", key, ownerNode)
			s.proxyRequest(w, r, ownerAddr)
			return
		}
	} else {
		log.Printf("Bypassing cluster for GET %s due to local=true\n", key)
	}

	val, found := s.Store.Get(key)
	if !found {
		http.Error(w, "Key not found", http.StatusNotFound)
		return
	}

	response, _ := json.Marshal(map[string]interface{}{"key": key, "value": val})
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Missing 'key' query parameter", http.StatusBadRequest)
		return
	}

	forceLocal := r.URL.Query().Get("local") == "true"

	if !forceLocal {
		ownerNode, ownerAddr, err := s.HashRing.GetOwner(key)
		if err != nil {
			http.Error(w, "Cluster error", http.StatusInternalServerError)
			return
		}

		if ownerNode != s.Cfg.NodeName {
			s.proxyRequest(w, r, ownerAddr)
			return
		}
	}

	s.Store.Delete(key)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleKeys(w http.ResponseWriter, r *http.Request) {
	// Only returns keys on THIS local node, for debugging
	keys := s.Store.Keys()
	response, _ := json.Marshal(keys)
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

func (s *Server) proxyRequest(w http.ResponseWriter, r *http.Request, targetAddr string) {
	url := fmt.Sprintf("http://%s%s", targetAddr, r.URL.String())

	bodyBytes, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	proxyReq, err := http.NewRequest(r.Method, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
		return
	}

	proxyReq.Header = r.Header

	resp, err := s.Client.Do(proxyReq)
	if err != nil {
		http.Error(w, "Proxy request failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
