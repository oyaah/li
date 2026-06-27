package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/oyaah/li/internal/output"
	"github.com/oyaah/li/internal/voyager"
)

func TestCurrentVersionInfoDefaults(t *testing.T) {
	info := currentVersionInfo()
	if info.Version != "dev" || info.Commit != "none" || info.Date != "unknown" {
		t.Fatalf("defaults = %+v", info)
	}
	if info.SchemaVersion != voyager.SchemaVersion {
		t.Fatalf("schema version = %q want %q", info.SchemaVersion, voyager.SchemaVersion)
	}
}

func TestVersionInfoPlainOutput(t *testing.T) {
	info := versionInfo{Version: "1.2.3", Commit: "abc", Date: "2026-06-28", SchemaVersion: "2026-06-27"}
	var stdout, stderr bytes.Buffer
	p := &output.Printer{Format: output.Plain, Out: &stdout, Err: &stderr}
	if err := p.Data(info); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), "1.2.3\tabc\t2026-06-28\t2026-06-27\n"; got != want {
		t.Fatalf("plain output = %q want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestVersionInfoJSONOutput(t *testing.T) {
	info := versionInfo{Version: "1.2.3", Commit: "abc", Date: "2026-06-28", SchemaVersion: "2026-06-27"}
	var stdout bytes.Buffer
	p := &output.Printer{Format: output.JSON, Out: &stdout}
	if err := p.Data(info); err != nil {
		t.Fatal(err)
	}
	var got versionInfo
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got != info {
		t.Fatalf("json output = %+v want %+v", got, info)
	}
}
