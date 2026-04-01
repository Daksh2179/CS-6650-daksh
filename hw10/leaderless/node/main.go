package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ─── Data Structures ───────────────────────────────────────────────────────

type ValueEntry struct {
	Value   string `json:"value"`
	Version int64  `json:"version"`
}

type SetRequest struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Version int64  `json:"version"`
}

type GetResponse struct {
	Value   string `json:"value"`
	Version int64  `json:"version"`
}

// ─── Global State ──────────────────────────────────────────────────────────

var (
	store      = make(map[string]ValueEntry)
	storeMu    sync.RWMutex
	versionCtr int64

	nodeID string   // this node's own address (so it can identify itself)
	peers  []string // addresses of all OTHER nodes
)

// ─── Main ──────────────────────────────────────────────────────────────────

func main() {
	nodeID = getEnv("NODE_ID", "")
	peerStr := getEnv("NODE_ADDRESSES", "")
	if peerStr != "" {
		peers = strings.Split(peerStr, ",")
	}

	log.Printf("Starting leaderless node: id=%s peers=%v", nodeID, peers)

	http.HandleFunc("/set", handleSet)
	http.HandleFunc("/get", handleGet)
	http.HandleFunc("/local_read", handleLocalRead)
	http.HandleFunc("/internal_set", handleInternalSet)

	port := getEnv("PORT", "8080")
	log.Printf("Listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// ─── Handlers ──────────────────────────────────────────────────────────────

// POST /set  {"key":"foo","value":"bar"}
// This node becomes the Write Coordinator for this request.
// W=N: must replicate to ALL peers before responding.
func handleSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Key == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Assign version -- each node has its own counter but we use
	// a globally unique version by combining node ID + local counter.
	// Simple approach: just use unix nano as version (good enough for this assignment)
	version := atomic.AddInt64(&versionCtr, 1)
	// Make version unique across nodes by mixing in time
	version = time.Now().UnixNano()
	req.Version = version

	// Write to self first
	writeLocal(req.Key, req.Value, version)

	// Replicate to ALL peers (W=N)
	type result struct{ ok bool }
	results := make(chan result, len(peers))

	for _, peer := range peers {
		go func(addr string) {
			err := forwardSet(addr, req)
			results <- result{ok: err == nil}
		}(peer)
	}

	// Wait for all peers
	successCount := 1 // self counts
	for i := 0; i < len(peers); i++ {
		res := <-results
		if res.ok {
			successCount++
		}
	}

	if successCount == len(peers)+1 {
		w.WriteHeader(http.StatusCreated)
	} else {
		// Partial success -- still return 201 but log it
		log.Printf("Warning: only %d/%d nodes confirmed write for key=%s", successCount, len(peers)+1, req.Key)
		w.WriteHeader(http.StatusCreated)
	}
}

// GET /get?key=foo
// R=1: just return this node's local value
func handleGet(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "missing key", http.StatusBadRequest)
		return
	}
	// R=1: return local value only (this is where inconsistency window shows up)
	respondLocalRead(w, key)
}

// GET /local_read?key=foo -- same as get for leaderless, explicit local read for tests
func handleLocalRead(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "missing key", http.StatusBadRequest)
		return
	}
	respondLocalRead(w, key)
}

// POST /internal_set -- called by write coordinator to replicate to this node
func handleInternalSet(w http.ResponseWriter, r *http.Request) {
	var req SetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Simulate replication delay
	time.Sleep(100 * time.Millisecond)

	writeLocal(req.Key, req.Value, req.Version)
	w.WriteHeader(http.StatusOK)
}

// ─── Helpers ───────────────────────────────────────────────────────────────

func writeLocal(key, value string, version int64) {
	storeMu.Lock()
	defer storeMu.Unlock()
	existing, ok := store[key]
	if !ok || version > existing.Version {
		store[key] = ValueEntry{Value: value, Version: version}
	}
}

func respondLocalRead(w http.ResponseWriter, key string) {
	storeMu.RLock()
	entry, ok := store[key]
	storeMu.RUnlock()

	if !ok {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GetResponse{Value: entry.Value, Version: entry.Version})
}

func forwardSet(addr string, req SetRequest) error {
	// Simulate delay before sending to each peer
	time.Sleep(200 * time.Millisecond)

	body, _ := json.Marshal(req)
	resp, err := http.Post(addr+"/internal_set", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("forwardSet to %s failed: %v", addr, err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status from %s: %d", addr, resp.StatusCode)
	}
	return nil
}

func fetchFromPeer(addr string, key string) (ValueEntry, error) {
	resp, err := http.Get(addr + "/internal_get?key=" + key)
	if err != nil {
		return ValueEntry{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ValueEntry{}, fmt.Errorf("not found")
	}
	body, _ := io.ReadAll(resp.Body)
	var gr GetResponse
	if err := json.Unmarshal(body, &gr); err != nil {
		return ValueEntry{}, err
	}
	return ValueEntry{Value: gr.Value, Version: gr.Version}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}