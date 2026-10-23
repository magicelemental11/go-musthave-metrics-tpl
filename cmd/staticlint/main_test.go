package main

import "testing"

func TestAnalyzers(t *testing.T) {
	names := map[string]bool{}
	for _, a := range analyzers() {
		if names[a.Name] {
			t.Errorf("duplicate analyzer %s", a.Name)
		}
		names[a.Name] = true
	}
	for _, want := range []string{"noosexit", "printf", "nilness", "SA1000", "S1000", "bodyclose", "nilerr"} {
		if !names[want] {
			t.Errorf("analyzer %s is missing", want)
		}
	}
}
