package main

import (
	"fmt"
	"log/slog"
	"net/rpc"
)

// This file is concered with all the network interactions that go on in Raft

type Server struct {
	Log slog.Logger
}

func NewServer() {

}

func (s *Server) Call(tcpAddr int, rpcName string, args interface{}, reply interface{}) (err error) {
	c, err := rpc.DialHTTP("unix", fmt.Sprintf("%d", tcpAddr))
	if err != nil {
		s.Log.Error("error dialing address=(%s)", slog.Int("peerAddr", tcpAddr))
		return err
	}
	defer c.Close()

	err = c.Call(rpcName, args, reply)
	if err != nil {
		s.Log.Debug("error calling RPC=(%v)", slog.Any("error", err))
		return err
	}

	return err
}

type RequestVoteArgs struct {
	Term int
	CandidateId int
}

type RequestVoteReply struct {
	Term int
	VoteGranted bool
}

type AppendEntriesArgs struct {
	Term int
	LeaderId int
	Entries []any
}

type AppendEntriesReply struct {
	// current term for the leader to update itself.
	Term int
	Successs bool
}