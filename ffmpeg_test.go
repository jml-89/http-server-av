package main

import (
	"testing"
)

func TestGetMetadata(t *testing.T) {
	pathTestFile := "/home/choleric/Videos/tmp/tmk.mp4"
	_, err := GetMetadata(pathTestFile)
	if err != nil {
		t.Fatalf("ERROR: %v\n", err)
	}
}

func TestCreateThumbnail(t *testing.T) {
	_, err := CreateThumbnail("/home/choleric/Videos/tmp/tmk.mp4")
	if err != nil {
		t.Fatalf("%v\n", err)
	}
}
