package noosexit_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/magicelemental11/go-musthave-metrics-tpl/cmd/staticlint/noosexit"
)

func TestAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), noosexit.Analyzer, "a", "b", "c", "d")
}
