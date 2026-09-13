package main

import "testing"

func TestParsePlatforms(t *testing.T) {
	all, err := parsePlatforms("")
	if err != nil || all != nil {
		t.Fatalf("empty = (%v, %v), want (nil, nil)", all, err)
	}
	got, err := parsePlatforms("greenhouse, workday")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 2 || got[0] != "greenhouse" || got[1] != "workday" {
		t.Fatalf("got %v", got)
	}
	if _, err := parsePlatforms("linkedin"); err == nil {
		t.Fatal("expected error for an unknown platform")
	}
}
