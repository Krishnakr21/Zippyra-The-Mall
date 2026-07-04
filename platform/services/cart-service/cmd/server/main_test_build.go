//go:build test
// +build test

package main

import "testing"

// testMainError stores the error from run() for testing
var testMainError error

func main() {
	testMainError = run()
	// In test mode, we don't call log.Fatal, just store the error
}

// GetTestMainError returns the error from the last main() execution (test only)
func GetTestMainError() error {
	return testMainError
}
