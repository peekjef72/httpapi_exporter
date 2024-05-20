//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

func init_sigusr2(user2 chan<- os.Signal) {
	signal.Notify(user2, syscall.SIGUSR2)
}
