package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/hashicorp/raft"
)

// Command is applied through Raft log
type Command struct {
	Op    string `json:"op"`
	Key   string `json:"key"`
	Value Value  `json:"value"`
}

type Value struct {
	Val string `json:"value"`
	Ts  int64  `json:"ts"`
}

type FSMSnapshot struct {
	store map[string]Value
}

func (fss FSMSnapshot) Persist(sink raft.SnapshotSink) error {
	b, _ := json.Marshal(fss.store)
	if _, err := sink.Write(b); err != nil {
		fmt.Printf("error while persisting data %v", err)
		sink.Cancel()
		return err
	}
	sink.Close()
	return nil
}

func (fss FSMSnapshot) Release() {

}

type FSM struct {
	mu   sync.RWMutex
	data map[string]Value
}

func NewFSM() *FSM {
	return &FSM{
		data: make(map[string]Value),
	}
}

func (f *FSM) Apply(l *raft.Log) interface{} {
	var c Command
	// copy the values from the log to command c
	json.Unmarshal(l.Data, &c)
	if c.Op == "set" {
		// acquire lock to ensure consistent write
		f.mu.Lock()
		f.data[c.Key] = c.Value
		f.mu.Unlock()
	}
	return nil
}

func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// shallow copy
	m := make(map[string]Value, len(f.data))
	for k, v := range f.data {
		m[k] = v
	}

	return &FSMSnapshot{store: m}, nil
}

func (f *FSM) Restore(rc io.ReadCloser) error {
	// decode stream into m
	var m map[string]Value
	if err := json.NewDecoder(rc).Decode(&m); err != nil {
		return err
	}

	// acquire lock to restore data cleanly
	f.mu.Lock()
	f.data = m
	f.mu.Unlock()
	return nil
}

func (f *FSM) Get(key string) (Value, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	v, ok := f.data[key]
	return v, ok
}
