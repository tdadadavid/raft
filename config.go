package main

import (
	"os"
	"time"
)

type Config struct {
	// Sets the time for an election to be begin
	ElectionTimeout time.Duration

	// Configures the time interval between each heartbeat from the leader to followers.
	HeartbeatTimer time.Duration
}

func getEnvOrDefault(key string, fallback any) (value interface{}) {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
} 