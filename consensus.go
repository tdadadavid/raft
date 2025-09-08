package main

import (
	"log"
	"math/rand"
	"sync"
	"time"
)

type RpcCalls string 

const (
	REQUEST_VOTE = "ConsensusModule.RequestVote"
)

type LogEntry struct {
	Command any
	Term    int
}

type State int

const (
	FOLLOWER State = 1
	CANDIDATE
	LEADER
	DEAD
)

type ConsesusModule struct {
	// peer id of current node
	id int

	// list of peers in the raft cluster
	peerIDs []int

	server *Server

	// Persistent State on all server
	currentTerm int
	votedFor    int
	log         []LogEntry

	// Volatile State on all servers
	state State
	electionResetEvent time.Time

	// handles concurrent access of the Consensus Module
	mu sync.Mutex
}

func (c *ConsesusModule) Log(info... any) {
	log.Printf(
		"RAFT - peer=[%s] - %v\n", info,
	)
}

// runElectionTimer This allow the peer to timeout and perform any operation need (run an election or vote for another peer)
//
// Behaviour
//  Firstly we get the current term and the timeout in the cluster. The timeout is randomly selected between 150-300ms 
//  We run checks against changes in state from Follower -> Candidate or vice versa, if there are changes then we stop the checks
//
//
func (c *ConsesusModule) runElectionTimer() {
	timeout := c.electionTimeout()
	
	// acquire lock to get current term
	c.mu.Lock()
	currentTerm := c.currentTerm
	c.mu.Unlock()
	c.Log("election timer started (%v), term=%d", timeout, currentTerm)


	// This controls the loop until
	// 1. The we find a leader 
	// 2. The timer expires and the peer becomes a candidate
	// Note: if the peer is already in candidate state it will keep running in the background
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		// block for every 10 * millisecond
		<-ticker.C

		// acquire lock to observe state
		c.mu.Lock()
		// Stop if peer is a leader, no need to run election timeout for leader peer.
		if c.state != CANDIDATE && c.state != FOLLOWER {
			c.Log("in election timer state=%s", c.state)
			c.mu.Unlock() // release lock
			return
		}

		// check if the term as changed during election-timeout waiting
		if currentTerm != c.currentTerm {
			c.Log("in election timer term changed from %d to %d", currentTerm, c.currentTerm)
			c.mu.Unlock() // release lock for other threads
			return
		}

		// Start election if 
		// 1. Peer has not heard from the leader
		// 2. Peer has not voted for another peer in the cluster.
		elaspsed := time.Since(c.electionResetEvent);
		if elaspsed >= timeout {
			c.startElection()
			c.mu.Unlock() // release lock after election
			return
		}

		// release lock at each iteration's end
		c.mu.Unlock()
	}
}

// startElection This starts an election if a peer has not heard from the leader or its vote has not been requested by the other peers 
//
// Behaviour
//  1. Switch to candidate
//  2. Increment term
//  4. Vote for yourself (because why not lol...😂)
//  3. Send RequestVote RPC to peers in the cluster & Wait for replies
//  4. Promote to Leader if peer get majority votes
func (c *ConsesusModule) startElection() {
	c.mu.Lock()
	// 1. Switch to candidate
	c.state = CANDIDATE

	// 2. Increment term
	c.currentTerm += 1
	newTerm := c.currentTerm

	// 3. Vote for self
	c.votedFor = c.id

	// Update last-time it heard from cluster because it just voted for itself.
	c.electionResetEvent = time.Now()
	c.mu.Unlock()
	c.Log("becomes CANDIDATE (newTerm=%d); current log enteries Log=%v", newTerm, c.log)

	votesReceived := 1

	// 4. request for vote from other peers
	for _, peerId := range c.peerIDs {
		go func (peerId int)  {
			args := RequestVoteArgs{
				Term: newTerm,
				CandidateId: c.id,
			}
			var reply RequestVoteReply

			// RPC request to peer requesting for vote
			if c.server.Call(peerId, REQUEST_VOTE, args, reply) {
				c.mu.Lock()
				defer c.mu.Unlock()
				c.Log("received RequestVoteReply, state = %v", c.state)

				// It is possible that while requesting for vote the peer becomes a LEADER
				// this can happen because 
				// 	1. Leader Appointment: Raft does not need all the participant's vote to pick a leader
				// 		in a cluster of 5 nodes if 3 nodes vote for the peer then it becomes the leader. 
				// 	2. Downgrade to Follower: It is possible that while the current peer is requesting for vote 
				// 		a new leader has been selected by the cluster and that leader has sent hearbeats to all peers 
				// 		registering its leadership
				// In both cases every other request for vote must be terminated and this is why checking state change is crucial. 
				if c.state != CANDIDATE {
					c.Log("State changed to = %s", c.state)
					return
				}

				// If Term returned from the peer is greater than the 'Term' in requesting peer
				// the requesting peer has to become a 'Follower' (Candidate -> Follower)
				if reply.Term > newTerm {
					c.Log("term is out of date with cluster's term, term=%s, cluster-term=%s", newTerm, reply.Term)
					// make requesting peer become a follower and update it's term with the cluster's current term
					c.becomeFollower(reply.Term)
					return
				} 

				// when the requesting peer and the replying peer have the same peer
				// 1. Check if the vote was granted
				// 2. If vote was granted check if it was, check if requesting peer already has Majority vote'
				if reply.Term == newTerm {
					if reply.VoteGranted {
						votesReceived++
						if c.hasMajority(votesReceived) {
							c.Log("election won with %d votes", votesReceived)
							c.startLeader() // elect requesting peer has leader
							return
						}
					}
				}
			}
		}(peerId)
	}

	// It is possible that for a term we don't have any leader meaning we did not have a 
	// succesful election, we need to start a timeout again to ensure another election on
	// another term starts.
	go c.runElectionTimer()
}

func (c *ConsesusModule) startLeader() {

}


// hasMajority checks if a peer has majority votes in a cluster
// 
// Behaviour:
//  using the formulat (2 * voteReceived) > (total no of peers + 1)
//  if we have 5 peers in a cluster and requesting peer has 3 votes then
//   >>> (2 * 3) > (4 + 1) = True
//  								^ 4 is the remaining number of peers (there are 5 peers, word-problem 🫠)
// 
// Returns
//  bool: has majority or does not 
func (c *ConsesusModule) hasMajority(voteCount int) bool {
	return voteCount * 2 > len(c.peerIDs) + 1
}

func (c *ConsesusModule) becomeFollower(term int) {
	c.mu.Lock()
	c.state = FOLLOWER
	c.currentTerm = term
	c.mu.Unlock()
}

// The timeout bandwidth for which a peer can wait before it changes to a candidate or leader
//
// Behaviour:
// 	The paper suggests picking a duration time between 150-300ms
//
// Return:
//  It returns a duration object (time.Duration)
func (c *ConsesusModule) electionTimeout() (t time.Duration) {
	return time.Duration(150+rand.Intn(15)) * time.Millisecond
}