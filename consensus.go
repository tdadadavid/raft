package main

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
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

type ConsensusModule struct {
	// peer id of current node
	id int

	// list of peers in the raft cluster
	peerIDs []int

	server *Server

	// contains configuration values for peers
	config *Config

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

func (c *ConsensusModule) Log(info... any) {
	log.Printf(
		"RAFT - peer=[%s] - %v\n", c.id, info,
	)
}

// runElectionTimer This allow the peer to timeout and perform any operation maybe run an election or vote for another peer
//
// Behaviour
//  Firstly we get the current term and the timeout in the cluster. The timeout is randomly selected between 150-300ms 
//  We run checks against changes in state from Follower -> Candidate or vice versa, if there are changes then we stop the checks
func (c *ConsensusModule) runElectionTimer() {
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

		// check if the term as changed during election-timeout waiting, stop the election timeout
		// if any change and move the next cluster's currentTerm
		if currentTerm != c.currentTerm {
			c.Log("in election timer term changed from %d to %d", currentTerm, c.currentTerm)
			c.mu.Unlock() // release lock for other threads
			return
		}

		// Start election if 
		// 1. Peer has not heard from the leader
		// 2. Peer has not voted for another peer in the cluster.
		elaspsedTime := time.Since(c.electionResetEvent);
		if elaspsedTime >= timeout {
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
// Note:
//  Every RPC calls is a gorountine because they're blocking (can take time for execution)
func (c *ConsensusModule) startElection() {
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

	// 4. request for vote from all other peers
	for _, peerId := range c.peerIDs {
		go func (peerId int)  {
			args := RequestVoteArgs{
				Term: newTerm,
				CandidateId: c.id,
			}
			var reply RequestVoteReply

			// RPC request to peer requesting for vote
			err := c.server.Call(peerId, "ConsensusModule.RequestVote", args, &reply)
			if err == nil {
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
							c.becomeLeader() // elect requesting peer has leader
							return
						}
					}
				}
			}
		}(peerId)
	}

	// It is possible that for a term we don't have any leader meaning we did not have a 
	// succesful election, we need to start a timeout again to ensure another election on another term starts.
	// If we had a succesful election then it will just return and wouldn't start any election timer.
	go c.runElectionTimer()
}

// becomeLeader
//
// Behaviour:
//  1. Makes the peer a leader
//  2. Begins to send heartbeats RPCs to follower peers
//  3. It stops all processes if leadership is lost.
//
// Note:
func (c *ConsensusModule) becomeLeader() {
	// 1. elect peer as leader
	c.state = LEADER
	c.Log("becomes LEADER; on term=%d, log=%v", c.currentTerm, c.log)

	// 2. spawn goroutine to send hearbeats to follower peers every 50 milliseconds 
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		for {
			// continuously send heartbeats messages to peers for every tick (tick = 50 milliseconds)
			c.sendHeartbeat()
			<-ticker.C

			// acquire lock to check if peer is still a leader . 
			c.mu.Lock()
			//3. If peer is not a Leader terminate process.
			if c.state != LEADER {
				c.mu.Unlock()
				return
			}
			c.mu.Unlock()
		}
	}()
}

// sendHeartbeat performs RPCs calls to follower peers
//
// Behaviour:
//  maSends RPCs call `AppendEntries` to follower peers
//
// Note:
// 	- Even though there might not be an entry the Raft algorithm we still send AppendEntry Rpc call
func (c *ConsensusModule) sendHeartbeat() {
	// 1. acquire lock to get current term
	c.mu.Lock()
	currentTerm := c.currentTerm
	c.mu.Unlock()

	// 2. For each peer send AppendEntries RPC 
	for _, peerId := range c.peerIDs {
		args := AppendEntriesArgs{
			Term: currentTerm,
			LeaderId: c.id,
		}

		// NOTE: We are wrapping every RPC call in a goroutine because we can have a flaky network and this would cause the request-response to block if run on the main thread
		go func (peerId int)  {
			var reply AppendEntriesReply
			c.Log("sending RPC AppendEntries to peer-%d, term=%d --- args=%v", peerId, currentTerm, args)
			err := c.server.Call(peerId, "ConsensusModule.AppendEntry", args, &reply)
			if err != nil {
				// TODO: continuously retry this particular peerr
				// FORNOW: just do a return return
				return
			}

			// NOTE: we acquired the lock after the RPC call not before or else it would block and reduce performance drastically
			c.mu.Lock()
			defer c.mu.Unlock()
			
			// 3. Checkif the term from the replying peer is greater than the current term (it means this leader is no longer the leader and would need to become a follower)
			if reply.Term > currentTerm {
				c.Log("Leadership lost, peer is on an old term. peerTerm=%d, latestTerm=%d", currentTerm, reply.Term)

				// 4. Update peer state to follower (it's obvious right 🫠)
				c.becomeFollower(reply.Term)
				return // this terminates the leader sending heartbeats
			}
		}(peerId)

	}
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
func (c *ConsensusModule) hasMajority(voteCount int) bool {
	return voteCount * 2 > len(c.peerIDs) + 1
}

func (c *ConsensusModule) becomeFollower(term int) {
	c.Log("becomes Follower on term=%d log=%v", term, c.log)
	c.mu.Lock()
	c.state = FOLLOWER
	c.currentTerm = term
	c.votedFor = -1 // reset voting state

	// If it becomes a Follower either by
	// 1. Loosing leadership, when the term for the current leader is stale compared to the cluster
	// 2. When leader has not sent any heartbeat
	c.electionResetEvent = time.Now()

	c.mu.Unlock()

	// start election timer.
	// once a peer becomes a follower let it start anticipating to begin an election if it recieves no heartbeat from the leader
	go c.runElectionTimer()
}

// The timeout bandwidth for which a peer can wait before it changes to a candidate or leader
//
// Behaviour:
// 	The paper suggests picking a duration time between 150-300ms
//
// Return:
//  It returns a duration object (time.Duration)
func (c *ConsensusModule) electionTimeout() (t time.Duration) {
	return time.Duration(150+rand.Intn(15)) * time.Millisecond
}

// RequestVote grants or denies a requesting peer its vote
//
// Behaviour:
//   Grants or denies vote to the requesting peer
//
// Return
//  error: every RPC handler must return an error
func (c *ConsensusModule) RequestVote(req RequestVoteArgs, reply *RequestVoteReply) (err error) {

	c.mu.Lock()
	defer c.mu.Unlock()

	// 1. Checks if the peer is alive (its possible a peer is down), if it isn't just stop processing
	if c.state == DEAD {
		//NOTE: When a peer is dead we keep retrying
		// FORNOW: Just send an error message.
		err = fmt.Errorf("error peer-%d is dead", c.id)
		return err
	}

	c.Log("RequestVote: %+v [currentTerm=%d, votedFor=%d]", req, c.currentTerm, c.votedFor)

	// 2. Compare terms between peers
	// if current term is out of date then just become a follower
	if c.currentTerm < req.Term {
		c.Log("peer-%d term is out of date", c.id)
		c.becomeFollower(req.Term)
	}

	// 3. Check if peer is same and if peer has not voted or has voted for requesting peer (candidateID)

	// prepare conditions for the if..else statement to read more like the white paper
	sameTerm := c.currentTerm == req.Term 
	hasNotVoted := c.votedFor == -1
	hasVotedForReqPeer := c.votedFor == req.CandidateId

	if sameTerm && (hasNotVoted || hasVotedForReqPeer) {
		reply.VoteGranted = true // grant vote
		c.votedFor = req.CandidateId // track peer voted for
		c.electionResetEvent = time.Now() // set reset election timeout 
	} else {
		reply.VoteGranted = false // deny vote
	}

	// 4. if vote is granted or denied respond to the requesting peer with the current term.
	reply.Term = c.currentTerm
	c.Log("RequestVote reply = %+v", reply)

	return err
}

// AppendEntries it is used to
//  1. Append entries from leaders to followers
//  2. Send heartbeats messages from leaders
//
// Behaviour:
//
// Returns:
// 	- Error
func (c *ConsensusModule) AppendEntries(req AppendEntriesArgs, reply *AppendEntriesReply) (err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// 1. Check the peer is alive
	if c.state == DEAD {
		return err
	}

	c.Log("AppendEntries: %+v", req.Entries)

	// 2. Check if the term is outdate
	if c.currentTerm < req.Term {
		c.Log("... term out of date in AppendEntries")

		// make peer a follower
		c.becomeFollower(req.Term)
	}

	reply.Successs = false

	// 4. Check if they're on the same term
	if req.Term == c.currentTerm {
		// 5. Check if the state of peer is a follower if not then make it a follower
		// NOTE: This happens when the current peer is performing an election and another peer has already won the election and is already sending heartbeats to the followers.
		if c.state != FOLLOWER {
			c.becomeFollower(req.Term) // downgrade peer to Follower from Candidate
		}
		c.electionResetEvent = time.Now()

		// Mark the operation as succesful
		reply.Successs = true
	}

	// Reply with current peer's term is operation was succesful or not
	reply.Term = c.currentTerm
	c.Log("AppendEntries reply = %+v", *reply)


	return err
}