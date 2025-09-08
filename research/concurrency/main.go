package main

// func main() {
// 	var count = 0;
// 	var finished = 1;

// 	var mu sync.Mutex
// 	cond := sync.NewCond(&mu)

// 	// This simulates a raft node seeking for vote
// 	// each vote is a new gorountine spawned
// 	for i := 0; i < 10; i++ {
// 		go func ()  {
// 			// request the vote
// 			vote := requestVote()

// 			// acquire lock to make sure every state change is done atomically
// 			mu.Lock()
// 			defer mu.Unlock()
// 			if vote {
// 				count++
// 			}
// 			finished++

// 			// notify all sleeping (waiting) threads to resume execution
// 			cond.Broadcast()
// 		}()
// 	}

// 	// acquire locks to make sure election verification is done atomically
// 	mu.Lock()

// 	for count < 5 && finished != 10 {
// 		cond.Wait() // wait on the condition is fulfilled then run the for-loop again
// 	}
// 	if count >= 5 {
// 		fmt.Println("received 5+ votes")
// 	} else {
// 		fmt.Println("lost")
// 	}
// 	mu.Unlock()
// }

func main() {
	Run()
}


// func requestVote() bool {
// 	flag := rand.Intn(2)
// 	return flag == 1
// }