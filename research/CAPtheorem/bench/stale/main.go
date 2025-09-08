package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

var (
	writeURL = flag.String("write-url", "http://127.0.0.1:9001", "write node")
	readURL  = flag.String("read-url", "http://127.0.0.1:9002", "read node")
	opsz      = flag.Int("ops", 1000, "number of operations")
)

type kvResponse struct {
	Val string `json:"value"`
	Ts  int64  `json:"ts"`
}

func main() {
	flag.Parse()
	staleCount := 0

	client := &http.Client{Timeout: 2 * time.Second}

	for i := 0; i < *opsz; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("val-%d", i)
		ts := time.Now().UnixNano()

		// write
		req, _ := http.NewRequest("POST", *writeURL+"/kv/"+key, strings.NewReader(value))
		r, err := client.Do(req)
		if err != nil {
			log.Printf("POST error wrting to raft cluster: error=%v statusCode=%d", err, r.StatusCode)
			return
		}

		// fmt.Printf("\nresponse=%v\n", string(stringRes))
		r.Body.Close()

		fmt.Printf("Write %s Read %s\n", *writeURL, *readURL)

		// immediate read from another node
		reqR, _ := http.NewRequest("GET", *readURL+"/kv/"+key, nil)
		resp, err := client.Do(reqR)
		if err != nil || resp.StatusCode >= 400 {
			stringRes, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Printf("unable to read body = %v", err)
				return
			}
			fmt.Printf("GET error from request err=%v statusCode=%d\n", string(stringRes), resp.StatusCode)
			staleCount++
			continue
		}

		var res kvResponse
		json.NewDecoder(resp.Body).Decode(&res)
		resp.Body.Close()

		// fmt.Printf("GET Response=%v\n", res)
		// fmt.Printf("ReadTime %d WriteTime %d", res.Ts, ts)

		if res.Ts < ts {
			staleCount++
		}
	}

	fmt.Printf("Total ops: %d, Stale reads: %d, Staleness %% = %.2f\n",
		*opsz, staleCount, float64(staleCount)/float64(*opsz)*100)
}
