package main

import (
	"strings"

	"github.com/gostaticanalysis/nilerr"
	"github.com/timakin/bodyclose/passes/bodyclose"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
	"golang.org/x/tools/go/analysis/passes/appends"
	"golang.org/x/tools/go/analysis/passes/assign"
	"golang.org/x/tools/go/analysis/passes/atomic"
	"golang.org/x/tools/go/analysis/passes/bools"
	"golang.org/x/tools/go/analysis/passes/copylock"
	"golang.org/x/tools/go/analysis/passes/defers"
	"golang.org/x/tools/go/analysis/passes/errorsas"
	"golang.org/x/tools/go/analysis/passes/httpresponse"
	"golang.org/x/tools/go/analysis/passes/loopclosure"
	"golang.org/x/tools/go/analysis/passes/lostcancel"
	"golang.org/x/tools/go/analysis/passes/nilfunc"
	"golang.org/x/tools/go/analysis/passes/nilness"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/shift"
	"golang.org/x/tools/go/analysis/passes/stdmethods"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"golang.org/x/tools/go/analysis/passes/tests"
	"golang.org/x/tools/go/analysis/passes/unmarshal"
	"golang.org/x/tools/go/analysis/passes/unreachable"
	"golang.org/x/tools/go/analysis/passes/unusedresult"
	"honnef.co/go/tools/simple"
	"honnef.co/go/tools/staticcheck"

	"github.com/magicelemental11/go-musthave-metrics-tpl/cmd/staticlint/noosexit"
)

const (
	staticcheckPrefix = "SA"
	simpleCheck       = "S1000"
)

func main() {
	multichecker.Main(analyzers()...)
}

func analyzers() []*analysis.Analyzer {
	checks := []*analysis.Analyzer{
		appends.Analyzer,
		assign.Analyzer,
		atomic.Analyzer,
		bools.Analyzer,
		copylock.Analyzer,
		defers.Analyzer,
		errorsas.Analyzer,
		httpresponse.Analyzer,
		loopclosure.Analyzer,
		lostcancel.Analyzer,
		nilfunc.Analyzer,
		nilness.Analyzer,
		printf.Analyzer,
		shift.Analyzer,
		stdmethods.Analyzer,
		structtag.Analyzer,
		tests.Analyzer,
		unmarshal.Analyzer,
		unreachable.Analyzer,
		unusedresult.Analyzer,
		bodyclose.Analyzer,
		nilerr.Analyzer,
		noosexit.Analyzer,
	}
	for _, a := range staticcheck.Analyzers {
		if strings.HasPrefix(a.Analyzer.Name, staticcheckPrefix) {
			checks = append(checks, a.Analyzer)
		}
	}
	for _, a := range simple.Analyzers {
		if a.Analyzer.Name == simpleCheck {
			checks = append(checks, a.Analyzer)
		}
	}
	return checks
}
