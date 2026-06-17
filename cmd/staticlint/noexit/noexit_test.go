package noexit_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/puzakov/watchdog/cmd/staticlint/noexit"
)

func TestNoExitAnalyzer(t *testing.T) {
	// Run analysistest on the testdata directory.
	// The testdata directory contains Go files with expected diagnostics
	// marked by "// want" comments.
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, noexit.Analyzer, "a", "b")
}
