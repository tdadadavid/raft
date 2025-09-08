package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

// PUT /kv/{key} body
// GET /kv/{key} -> gets value

func main() {

	// read environment vars
	nodeID := os.Getenv("NODE_ID")
	raftBind := os.Getenv("RAFT_BIND")
	httpBind := os.Getenv("HTTP_BIND")
	// peers := os.Getenv("PEERS")

	// validations
	if nodeID == "" {
		nodeID = "1"
	}
	if raftBind == "" {
		raftBind = "127.0.0.1:1200" + nodeID
	}
	if httpBind == "" {
		httpBind = "127.0.0.1:800" + nodeID
	}

	dataDir := filepath.Join("./data", fmt.Sprintf("node%s", nodeID))
	// Ensure a clean data directory for development/testing to avoid 'log not found' errors.
	if err := os.RemoveAll(dataDir); err != nil && !os.IsNotExist(err) {
		panic(err)
	}

	// 755 or 0755 means folder will readable and executable by other but only writable by USER
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		panic(err)
	}

	// raft setup
	addr, _ := net.ResolveTCPAddr("tcp", raftBind)
	transport, err := raft.NewTCPTransport(raftBind, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		log.Fatalf("raft transport: %v", err)
	}

	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID("node" + nodeID)
	config.Logger = hclog.New(&hclog.LoggerOptions{
		Name:   "[Raft]",
		Level:  hclog.Info,
		Output: os.Stderr,
	})

	snapshotStore := raft.NewInmemSnapshotStore()
	boltStore, err := raftboltdb.NewBoltStore(filepath.Join(dataDir, "raft.db"))
	if err != nil {
		log.Fatalf("bolt store: %v", err)
	}

	logStore := boltStore

	fsm := NewFSM()

	r, err := raft.NewRaft(config, fsm, logStore, logStore, snapshotStore, transport)
	if err != nil {
		log.Fatalf("new raft: %v", err)
	}

	hasState, err := raft.HasExistingState(logStore, logStore, snapshotStore)
	if err != nil {
		log.Fatalf("error checking raft state: %v", err)
	}

	if nodeID != "1" {
		joinReq := map[string]string{
			"id":      "node" + nodeID,
			"address": raftBind,
		}
		b, err := json.Marshal(joinReq)
		if err != nil {
			log.Printf("unable to marshal json")
		}

		resp, err := http.Post("http://127.0.0.1:8001/join", "application/json", strings.NewReader(string(b)))
		if err != nil {
			log.Printf("failed to join cluster %v", err)
		} else {
			resp.Body.Close()
		}
	}

	if !hasState && nodeID == "1" {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      "node1",
					Address: "127.0.0.1:12001",
				},
				{
					ID:      "node2",
					Address: "127.0.0.1:12002",
				},
				{
					ID:      "node3",
					Address: "127.0.0.1:12003",
				},
				{
					ID:      "node4",
					Address: "127.0.0.1:12004",
				},
			},
		}
		future := r.BootstrapCluster(configuration)
		if err := future.Error(); err != nil {
			log.Printf("bootstrap error, server might already being bootstrapped %v", err)
		}
	}

	// HTTP API
	mux := http.NewServeMux()
	mux.HandleFunc("/kv/", func(w http.ResponseWriter, req *http.Request) {
		// get the key
		key := strings.TrimPrefix(req.URL.Path, "/kv/")

		// operation check
		switch req.Method {
		// write operation
		case http.MethodPost:
			// writes must only happen on leaders
			// leader redirection
			if r.State() != raft.Leader {
				leader := r.Leader()
				if leader == "" {
					http.Error(w, "error: no available leader", http.StatusServiceUnavailable)
					return
				}

				// Proxy the request to the leader, this will make all nodes act like a leader
				resp, err := http.DefaultClient.Do(req.Clone(req.Context()))
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadGateway)
					return
				}
				defer resp.Body.Close()

				w.WriteHeader(resp.StatusCode)
				io.Copy(w, resp.Body)
				return

			}

			body, err := io.ReadAll(req.Body)
			if err != nil {
				log.Printf("error reading request body: %v", err)
				http.Error(w, fmt.Sprintf("error reading request body %v", err), http.StatusUnauthorized)
				return
			}

			cmd := &Command{
				Op:  "set",
				Key: key,
				Value: Value{
					Val: string(body),
					Ts:  time.Now().UnixNano(),
				},
			}
			b, _ := json.Marshal(cmd)
			future := r.Apply(b, 5*time.Second)
			if err := future.Error(); err != nil {
				log.Printf("error applying changes to cluster: %v", err)
				http.Error(w, fmt.Sprintf("error applying changes to raft cluster %v", err), http.StatusBadGateway)
				return
			}

			w.Write([]byte("Ok"))
		// read operation
		case http.MethodGet:
			// ensure the local fsm node is synced up with the leader node before responding to avoid stale results
			if err := r.Barrier(1 * time.Second).Error(); err != nil {
				http.Error(w, fmt.Sprintf("error creating barrier %v", err), http.StatusForbidden)
				return
			}

			// get the value using the key
			v, ok := fsm.Get(key)
			if !ok {
				http.Error(w, fmt.Sprintf("error reading key %s from node: err=%v", key, err), http.StatusNotFound)
				return
			}

			resp, err := json.Marshal(v)
			if err != nil {
				http.Error(w, fmt.Sprintf("error marshalling response: err=%v", err), http.StatusNotFound)
				return
			}

			// send response back
			w.Write(resp)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/join", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			http.Error(w, "only post is allowed", http.StatusMethodNotAllowed)
			return
		}

		// none leader nodes sends request to join the leader
		type JoinRequest struct {
			ID      string `json:"id"`
			Address string `json:"address"`
		}

		var jr JoinRequest
		if err := json.NewDecoder(req.Body).Decode(&jr); err != nil {
			http.Error(w, fmt.Sprintf("error adding voter %v", err), http.StatusInternalServerError)
			return
		}
		log.Printf("node %s at address %s request to join leader", jr.ID, jr.Address)

		// add server to raft cluster
		future := r.AddVoter(raft.ServerID(jr.ID), raft.ServerAddress(jr.Address), 0, 0)
		if err := future.Error(); err != nil {
			http.Error(w, fmt.Sprintf("error adding voter %v", err), http.StatusInternalServerError)
			return
		}

		log.Printf("node %s at %s address added to cluster", jr.ID, jr.Address)
		w.WriteHeader(http.StatusNoContent)
	})

	// serve http
	log.Printf("HTTP listening on %s, Raft on %s", httpBind, raftBind)
	log.Fatal(http.ListenAndServe(httpBind, mux))
}
