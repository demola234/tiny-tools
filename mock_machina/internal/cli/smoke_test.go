package cli_test

import "testing"

// TestCISmoke fails on purpose to prove CI goes red. Reverted in the next commit.
func TestCISmoke(t *testing.T) {
	t.Parallel()
	t.Fatal("ci smoke")
}
