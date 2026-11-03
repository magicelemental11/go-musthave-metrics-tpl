package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/packages"
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
		if err = os.WriteFile(golden, got, 0o644); err != nil {
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

func TestPackageDir(t *testing.T) {
	tests := []struct {
		name    string
		files   []string
		want    string
		wantErr bool
	}{
		{name: "no files", files: nil, want: ""},
		{name: "one directory", files: []string{"/m/a/x.go", "/m/a/y.go"}, want: "/m/a"},
		{name: "several directories", files: []string{"/m/a/x.go", "/m/b/y.go"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := packageDir(&packages.Package{PkgPath: "m/a", GoFiles: tt.files})
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("dir = %q, want %q", got, tt.want)
			}
		})
	}
}
