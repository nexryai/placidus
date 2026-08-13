package main

import "testing"

func TestSanitizedHostRemovesCredentialsAndPath(t *testing.T) {
	got := sanitizedHost("http://user:secret@mirakurun.example:40772/private")
	if got != "mirakurun.example:40772" {
		t.Fatalf("sanitizedHost() = %q", got)
	}
}
