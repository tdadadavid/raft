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

func (s *Server) Call(tcpAddr int, rpcName string, args interface{}, reply interface{}) bool {
	c, err := rpc.DialHTTP("unix", fmt.Sprintf("%d", tcpAddr))
	if err != nil {
		s.Log.Error("error dialing address=(%s)", slog.Int("peerAddr", tcpAddr))
		return false
	}
	defer c.Close()

	err = c.Call(rpcName, args, reply)
	if err != nil {
		s.Log.Debug("error calling RPC=(%v)", slog.Any("error", err))
		return false
	}

	return true
}

type RequestVoteArgs struct {
	Term int
	CandidateId int
}

type RequestVoteReply struct {
	Term int
	VoteGranted bool
}