package main

import (
	"log"
	"os"
	"sync/atomic"
)

var (
	devMode bool
	logSeq  atomic.Int64
)

func init() {
	devMode = os.Getenv("TCFORGE_DEV") == "1"
	if devMode {
		log.SetFlags(log.Ltime | log.Lmicroseconds)
	} else {
		log.SetFlags(0)
	}
}

// dlog prints a numbered log line in dev mode; no-op in prod.
func dlog(format string, args ...any) {
	if !devMode {
		return
	}
	n := logSeq.Add(1)
	log.Printf("[#%d] "+format, append([]any{n}, args...)...)
}
