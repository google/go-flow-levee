package config

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSetConfig(t *testing.T) {
	set := &Config{ReportMessage: "test"}
	SetConfig(set)
	read, err := ReadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff := cmp.Diff(set, read); diff != "" {
		t.Errorf("set config differs from read config (-set, +read):\n%s", diff)
	}
}
