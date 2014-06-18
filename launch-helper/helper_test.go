package main

import "fmt"
import "testing"
import "time"

func TestLaunch(t *testing.T) {
	StartHelper = FakeStart
	StopHelper = FakeStop
	commands := make(chan []string)
	commandList := [][]string{
		[]string{"foo1", "bar1", "bat1", "baz1"},
		[]string{"foo2", "bar2", "bat2", "baz2"},
	}

	go runner(commands)
	for _, command := range commandList {
		fmt.Printf("sending %s\n", command)
		commands <- command
	}
	time.Sleep(4 * 1e9)
}
