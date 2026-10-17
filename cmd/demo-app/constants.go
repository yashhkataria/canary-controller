package main

import "time"

const (
	addr              = ":8080"
	shutdownTimeout   = 10 * time.Second
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeoutExtra = 10 * time.Second
	idleTimeout       = 60 * time.Second
)
