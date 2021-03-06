package util

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type Change struct {
	BlockID int
	Data    string
	Success bool
}

//Based on "timer" prorty from config file
//Schedule goroutine that will ping other goroutines via send channel
func Schedule(send chan bool, duration string) {
	u, err := time.ParseDuration(duration)
	if err != nil {
		log.Fatalf("Couldn't set a scheduler due to improper time format: %s\n", duration)
	}
	for range time.Tick(u) {
		send <- true
	}
}

//Run any arbitrary POSIX shell command
func RunCmd(blockID int, send chan Change, rec chan bool, action map[string]interface{}) {
	cmdStr := action["command"].(string)
	run := true

	for run {
		out, err := exec.Command("sh", "-c", cmdStr).Output()
		if err != nil {
			send <- Change{blockID, err.Error(), false}
		}
		send <- Change{blockID, strings.TrimSuffix(string(out), "\n"), true}
		//Block until other thread pings
		run = <-rec
	}
}

//Create a channel that captures all 34-64 signals
func GetSIGRTchannel() chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	sigArr := make([]os.Signal, 31)
	for i := range sigArr {
		sigArr[i] = syscall.Signal(i + 0x22)
	}
	signal.Notify(sigChan, sigArr...)
	return sigChan
}
