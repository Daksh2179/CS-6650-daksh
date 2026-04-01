package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Data Structures

type ValueEntry struct {
	Value   string `json:"value"`
	Version int64  `json:"version"`
}

type SetRequest struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Version int64  `json:"version"` // used when leader forwards to follower
}

type GetResponse struct {
	Value   string `json:"value"`
	Version int64  `json:"version"`
}

// Global State 

var (
	store      = make(map[string]ValueEntry)
	storeMu    sync.RWMutex
	versionCtr int64 // atomic version counter

	role        string   // "leader" or "follower"
	peers       []string // addresses of other nodes
	W           int      // write quorum
	R           int      // read quorum
)

// Main

func main() {
	role = getEnv("ROLE", "follower")
	W, _ = strconv.Atoi(getEnv("W", "5"))
	R, _ = strconv.Atoi(getEnv("R", "1"))

	peerStr := getEnv("NODE_ADDRESSES", "")
	if peerStr != "" {
		peers = strings.Split(peerStr, ",")
	}

	log.Printf("Starting node: role=%s W=%d R=%d peers=%v", role, W, R, peers)

	http.HandleFunc("/set", handleSet)
	http.HandleFunc("/get", handleGet)
	http.HandleFunc("/local_read", handleLocalRead)
	http.HandleFunc("/internal_set", handleInternalSet) // leader -> follower
	http.HandleFunc("/internal_get", handleInternalGet) // leader -> follower read

	port := getEnv("PORT", "8080")
	log.Printf("Listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

//  Handlers 

// POST /set  {"key":"foo","value":"bar"}
func handleSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if role != "leader" {
		http.Error(w, "not the leader", http.StatusForbidden)
		return
	}

	var req SetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Key == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Assign a new version
	version := atomic.AddInt64(&versionCtr, 1)
	req.Version = version

	// Write to self first (counts as 1 write)
	writeLocal(req.Key, req.Value, version)
	successCount := 1

	if W == 1 {
		// W=1: respond immediately, replicate async
		w.WriteHeader(http.StatusCreated)
		go replicateToAll(req)
		return
	}

	// W>1: replicate synchronously and wait for quorum
	type result struct{ ok bool }
	results := make(chan result, len(peers))

	for _, peer := range peers {
		go func(addr string) {
			err := forwardSet(addr, req)
			results <- result{ok: err == nil}
		}(peer)
	}

	// Collect until we hit W or exhaust all peers
	responded := 0
	for responded < len(peers) && successCount < W {
		res := <-results
		responded++
		if res.ok {
			successCount++
		}
	}

	if successCount >= W {
		w.WriteHeader(http.StatusCreated)
	} else {
		http.Error(w, "write quorum not reached", http.StatusInternalServerError)
	}

	// Keep draining in background (don't leave goroutines blocked)
	go func() {
		for responded < len(peers) {
			<-results
			responded++
		}
	}()
}

// GET /get?key=foo
func handleGet(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "missing key", http.StatusBadRequest)
		return
	}

	if role == "follower" {
		// Follower: sleep then return local value
		time.Sleep(50 * time.Millisecond)
		respondLocalRead(w, key)
		return
	}

	// Leader handles read based on R
	if R == 1 {
		// Just return local value
		respondLocalRead(w, key)
		return
	}

	// R>1: collect from R nodes (self + peers) and return most recent version
	type nodeResult struct {
		entry ValueEntry
		ok    bool
	}
	results := make(chan nodeResult, len(peers)+1)

	// Read from self
	go func() {
		storeMu.RLock()
		e, ok := store[key]
		storeMu.RUnlock()
		results <- nodeResult{entry: e, ok: ok}
	}()

	// Read from peers
	for _, peer := range peers {
		go func(addr string) {
			e, err := fetchFromPeer(addr, key)
			results <- nodeResult{entry: e, ok: err == nil}
		}(peer)
	}

	// Collect R responses
	best := ValueEntry{Version: -1}
	collected := 0
	total := len(peers) + 1

	for collected < total {
		res := <-results
		collected++
		if res.ok && res.entry.Version > best.Version {
			best = res.entry
		}
		if collected >= R {
			break
		}
	}

	if best.Version == -1 {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GetResponse{Value: best.Value, Version: best.Version})
}

// GET /local_read?key=foo  -- returns only this node's local value, no replication
func handleLocalRead(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "missing key", http.StatusBadRequest)
		return
	}
	respondLocalRead(w, key)
}

// POST /internal_set  -- called by leader to update this follower
func handleInternalSet(w http.ResponseWriter, r *http.Request) {
	var req SetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Simulate follower write delay
	time.Sleep(100 * time.Millisecond)

	writeLocal(req.Key, req.Value, req.Version)
	w.WriteHeader(http.StatusOK)
}

// GET /internal_get?key=foo "called by leader to read from this follower"
func handleInternalGet(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "missing key", http.StatusBadRequest)
		return
	}

	// Simulate follower read delay
	time.Sleep(50 * time.Millisecond)

	respondLocalRead(w, key)
}

//  Helpers 

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

func replicateToAll(req SetRequest) {
	for _, peer := range peers {
		time.Sleep(200 * time.Millisecond) // simulate replication delay per peer
		forwardSet(peer, req)
	}
}

func forwardSet(addr string, req SetRequest) error {
	time.Sleep(200 * time.Millisecond) // simulate delay before each follower message

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