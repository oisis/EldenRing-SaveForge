package main

import "testing"

func TestResolveQty_ClampsAboveMax(t *testing.T) {
	if got := resolveQty(1000, 999); got != 999 {
		t.Fatalf("resolveQty(1000, 999) = %d, want 999", got)
	}
}
