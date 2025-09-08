package main

import (
	"fmt"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/multi"
)

var NodeIDCounter = 0
var LinkIDCounter = 1

type Operator string
type Event string


type StateMachine struct {
	PresentState State
	PreviousState State
	g            *multi.DirectedGraph
}

func NewFSM() (s *StateMachine) {
	s = &StateMachine{}
	s.g = multi.NewDirectedGraph()

	return s
}

// StartState marks this value as the initial state in the FSM
//
// Behaviour
// This creates the present state and add it to the graph as a `Node`
// it then increments the node id tracker
//
// Example:
//
//	 fsm := New()
//	 fsm.StartState("A")
//
// Returns
// State: current state of the machine
func (s *StateMachine) StartState(initStateVal interface{}) State {
	// Set the initial State to the present State
	s.PresentState = State{
		Id:    int64(NodeIDCounter),
		Value: initStateVal,
	}

	// add state to the graph nodes.
	s.g.AddNode(s.PresentState)

	// increment the node
	NodeIDCounter++

	return s.PresentState
}

// AddState add a new state to the machine
//
// Behaviour
//
//	Create a state and adds it to the internal graph as a node, increments the node counter and returns the state.
//
// Example:
//
//	 fsm := New()
//		fsm.AddState("B")
//
// Returns
// State: The state that just got added
func (s *StateMachine) AddState(value interface{}) State {
	state := State{
		Id:    int64(NodeIDCounter),
		Value: value,
	}

	s.g.AddNode(state)

	NodeIDCounter++

	return state
}

// ConnectStates links two state together using a rule that determines the transition between them
//
// Behaviour:
//
//	It creates an edge between the source node (s1) and destination node (s2) using the rule, then increments the line counter.
func (s *StateMachine) ConnectStates(s1, s2 State, events Event) {
	s.g.SetLine(Link{
		Src:   s1,
		Dest:  s2,
		Id:    int64(LinkIDCounter),
		Events: []Event{events},
	})

	LinkIDCounter++
}


// FireEvent triggers an event in the machine causing state transitions
//
// Returns
//  Error: an error is returned if provided operator is not found
func (s *StateMachine) FireEvent(e Event) (err error) {
	// get the present state
	presentState := s.PresentState

	// get an iterator from the present state
	it := s.g.From(presentState.ID())

	for it.Next() {
		// get the next state from the current state.
		nextState := s.g.Node(it.Node().ID()).(State)

		// get edges connecting the `presentState` to the `nextState`.
		// NOTE: there is only one edge connecting two states in this FSM
		edges := graph.LinesOf(s.g.Lines(presentState.ID(), nextState.ID()))
		connectingEdge := edges[0].(Link)

		// for each edge's rules get the operator and perform the operation
		for _, event := range connectingEdge.Events {
			if event == e {
					// then perform a transition
					s.PreviousState = s.PresentState
					s.PresentState = nextState
					return err
				}
		}
	}

	return err
}

func (s *StateMachine) compute(events []string, printState bool) {
	for _, e := range events {
		s.FireEvent(Event(e))
		if printState {
			fmt.Printf("New state transition form Current: %s to %s ----  Event=%s\n", s.PreviousState.String(), s.PresentState.String(), e)
		}
	}
}

func (s *StateMachine) ComputeAndPrint(events []string)  {
	s.compute(events, true)
}
