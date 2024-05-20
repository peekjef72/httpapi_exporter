//go:build windows

package main

import (
	"os"
)

func init_sigusr2(user2 chan<- os.Signal) {
}
