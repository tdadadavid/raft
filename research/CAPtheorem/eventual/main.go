package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	store = struct {
		sync.RWMutex
		m map[string]Value
	}{m: make(map[string]Value)}
	peers  []string
	nodeID string
)

type Value struct {
	Val string `json:"val"`
	Ts  int64  `json:"ts"`
}

func main() {
	nodeID = os.Getenv("NODE_ID")
	httpBind := os.Getenv("HTTP_BIND")
	if httpBind == "" {
		httpBind = ":9001"
	}
	p := os.Getenv("PEERS")
	if p != "" {
		peers = strings.Split(p, ",")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/kv/", kvHandler)
	mux.HandleFunc("/internal/replicate", replicateHandler)

	log.Printf("eventual node %s listening %s", nodeID, httpBind)
	log.Fatal(http.ListenAndServe(httpBind, mux))
}

func kvHandler(w http.ResponseWriter, r *http.Request) {
	// extract the key
	key := strings.TrimPrefix(r.URL.Path, "/kv/")

	switch r.Method {
	// read operation
	case http.MethodPut:
		// read the value
		body, _ := io.ReadAll(r.Body)
		v := Value{
			Val: string(body),
			Ts:  time.Now().UnixNano(),
		}
		// acquire lock and add entry into the store
		store.Lock()
		store.m[key] = v
		store.Unlock()

		// replicate the entry to other peers in the cluster
		// this is like a fire-and-forget scenario.
		go replicateToPeers(key, v)

		store.RLock()
		re, _ := json.Marshal(store.m)
		store.RUnlock()

		// response
		w.Write(re)

	// read operation
	case http.MethodGet:
		// get the value from the store.
		store.RLock()
		v, ok := store.m[key]
		store.RUnlock()

		if !ok {
			http.NotFound(w, r)
			return
		}

		b, _ := json.Marshal(v)
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func replicateHandler(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Key, Val string
		Ts       int64
	}
	json.NewDecoder(r.Body).Decode(&in)

	store.Lock()
	cur, ok := store.m[in.Key]
	if !ok || in.Ts > cur.Ts {
		store.m[in.Key] = Value{Val: in.Val, Ts: in.Ts}
	}
	store.Unlock()

	w.WriteHeader(http.StatusNoContent)
}

func replicateToPeers(key string, value Value) {
	// byte value preparation
	b, _ := json.Marshal(map[string]interface{}{
		"key":   key,
		"value": value.Val,
		"ts":    value.Ts,
	})

	// peers replication
	for _, p := range peers {
		url := fmt.Sprintf("http://%s/internal/replicate", p)
		log.Printf("===Replicating===")
		// fire-and-forget
		http.Post(url, "application/json", strings.NewReader(string(b)))
	}
}
