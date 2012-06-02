package main

import (
	"bytes"
	"io/ioutil"
	"testing"
)

func TestSSIRelative(t *testing.T) {
	got, err := processSSI(".", "testdata/input.shtml")
	if err != nil {
		t.Fatalf("Error processing SSI: %v", err)
	}
	expected, err := ioutil.ReadFile("testdata/expected.html")
	if err != nil {
		t.Fatalf("Error reading expected data: %v", err)
	}
	if !bytes.Equal(got, expected) {
		t.Fatalf("Expected %s\n-- got --\n%s", expected, got)
	}
}

func TestSSIAbsolute(t *testing.T) {
	got, err := processSSI("testdata", "testdata/abs.shtml")
	if err != nil {
		t.Fatalf("Error processing SSI: %v", err)
	}
	expected, err := ioutil.ReadFile("testdata/expected.html")
	if err != nil {
		t.Fatalf("Error reading expected data: %v", err)
	}
	if !bytes.Equal(got, expected) {
		t.Fatalf("Expected %s\n-- got --\n%s", expected, got)
	}
}
