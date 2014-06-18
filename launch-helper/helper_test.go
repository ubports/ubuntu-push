package main

import "testing"

func TestLongRunningHelper(t *testing.T) {
	StartHelper = FakeStartLongLivedHelper
	StopHelper = FakeStop
	command :=[]string{"foo1", "bar1", "bat1", "baz1"}
	if runner(command) != helper_stopped {
		t.Fatalf("Long running helper is not stopped")
	}
}

func TestShortRunningHelper(t *testing.T) {
	StartHelper = FakeStartShortLivedHelper
	StopHelper = FakeStop
	command :=[]string{"foo1", "bar1", "bat1", "baz1"}
	if runner(command) != helper_finished {
		t.Fatalf("Short running helper doesn't finish")
	}
}
