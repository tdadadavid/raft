package main

import (
	"fmt"
	"strconv"

	"gonum.org/v1/gonum/graph"
)

// State is the data strcture that contains information about the machine
// In graph terminology this represents `Nodes` in graph
type State struct {
	Id    int64
	Value interface{}
}

// ID returns the ID of the current state in the Finite-State Machine
func (s State) ID() int64 {
	return s.Id
}

func (s State) String() string {
	switch s.Value.(type) {
	case int:
		return strconv.Itoa(s.Value.(int))
	case float32:
		return fmt.Sprintf("%f", s.Value.(float32))
	case float64:
		return fmt.Sprintf("%f", s.Value.(float64))
	case bool:
		return strconv.FormatBool(s.Value.(bool))
	case string:
		return s.Value.(string)
	default:
		return ""
	}
}

// Link is the data structure that connect two states (nodes) in the machine.
// In graph lingo this is represents the `edge` of nodes in the grpah.
type Link struct {
	Id        int64
	Src, Dest graph.Node
	Events     []Event
}

// ID returns the id of the current link or edge as you would like to call it.
func (l Link) ID() int64 {
	return l.Id
}

// From returns the source of the current node in the graph
func (l Link) From() graph.Node {
	return l.Src
}

// To returns the destination from the current node
//
// Example:
//
//	link := NewLink()
//	fmt.Printf("%v\n", link.To())
func (l Link) To() graph.Node {
	return l.Dest
}

func (l Link) ReversedLine() graph.Line {
	return Link{Src: l.Dest, Dest: l.Src}
}
