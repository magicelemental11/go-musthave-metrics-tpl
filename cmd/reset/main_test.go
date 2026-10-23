package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

func TestGenerateGolden(t *testing.T) {
	out, err := generate(".", "./testdata/basic")
	if err != nil {
		t.Fatal(err)
	}
	dir, err := filepath.Abs("testdata/basic")
	if err != nil {
		t.Fatal(err)
	}
	got, ok := out.files[filepath.Join(dir, genFile)]
	if !ok {
		t.Fatalf("no output for %s, got %v", dir, out.files)
	}
	golden := filepath.Join(dir, "reset.golden")
	if *update {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("generated code differs from golden file\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestGenerateRemovesStaleFile(t *testing.T) {
	out, err := generate(".", "./testdata/stale")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.files) != 0 {
		t.Errorf("unexpected output: %v", out.files)
	}
	want, err := filepath.Abs("testdata/stale/" + genFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.stale) != 1 || out.stale[0] != want {
		t.Errorf("stale = %v, want [%s]", out.stale, want)
	}
}
