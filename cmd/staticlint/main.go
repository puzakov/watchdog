// Command staticlint implements a multi-analyzer static analysis tool.
//
// It combines several categories of analyzers:
//   - Standard analyzers from golang.org/x/tools/go/analysis/passes
//   - All SA (static analysis) class analyzers from staticcheck.io
//   - Selected analyzers from other staticcheck.io classes (S, ST, QF)
//   - Third-party public analyzers
//   - A custom noexit analyzer that checks for os.Exit calls in main.main
//
// Usage:
//
//	staticlint ./...          # analyze all packages in the current module
//	staticlint ./cmd/server   # analyze a specific package
//	staticlint -noexit.exit   # disable the noexit analyzer
//
// To see which analyzers are enabled and their flags:
//
//	staticlint -help
package main

import (
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
	"golang.org/x/tools/go/analysis/passes/appends"
	"golang.org/x/tools/go/analysis/passes/assign"
	"golang.org/x/tools/go/analysis/passes/atomic"
	"golang.org/x/tools/go/analysis/passes/bools"
	"golang.org/x/tools/go/analysis/passes/copylock"
	"golang.org/x/tools/go/analysis/passes/errorsas"
	"golang.org/x/tools/go/analysis/passes/httpresponse"
	"golang.org/x/tools/go/analysis/passes/loopclosure"
	"golang.org/x/tools/go/analysis/passes/lostcancel"
	"golang.org/x/tools/go/analysis/passes/nilfunc"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/shadow"
	"golang.org/x/tools/go/analysis/passes/shift"
	"golang.org/x/tools/go/analysis/passes/stdmethods"
	"golang.org/x/tools/go/analysis/passes/stringintconv"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"golang.org/x/tools/go/analysis/passes/unreachable"

	"github.com/gostaticanalysis/nilerr"
	"github.com/puzakov/watchdog/cmd/staticlint/noexit"
	"honnef.co/go/tools/quickfix"
	"honnef.co/go/tools/simple"
	"honnef.co/go/tools/staticcheck"
	"honnef.co/go/tools/stylecheck"
)

func main() {
	// Collect all analyzers into a single slice
	checks := []*analysis.Analyzer{
		// Standard static analyzers from golang.org/x/tools/go/analysis/passes
		appends.Analyzer,
		assign.Analyzer,
		atomic.Analyzer,
		bools.Analyzer,
		copylock.Analyzer,
		errorsas.Analyzer,
		httpresponse.Analyzer,
		loopclosure.Analyzer,
		lostcancel.Analyzer,
		nilfunc.Analyzer,
		printf.Analyzer,
		shadow.Analyzer,
		shift.Analyzer,
		stdmethods.Analyzer,
		stringintconv.Analyzer,
		structtag.Analyzer,
		unreachable.Analyzer,

		// All SA (static analysis) class analyzers from staticcheck.io
		// These cover a wide range of common Go issues.
	}

	// Add all SA-class analyzers from staticcheck
	for _, a := range staticcheck.Analyzers {
		checks = append(checks, a.Analyzer)
	}

	// Add selected analyzers from other staticcheck classes

	// S (simplification) — suggests code simplifications
	// S1000 — all simplification checks
	for _, a := range simple.Analyzers {
		checks = append(checks, a.Analyzer)
	}

	// ST (style) — checks code style conventions
	for _, a := range stylecheck.Analyzers {
		checks = append(checks, a.Analyzer)
	}

	// QF (quickfix) — suggests quick fixes
	for _, a := range quickfix.Analyzers {
		checks = append(checks, a.Analyzer)
	}

	// Third-party public analyzers

	// nilerr checks for returning nil error when the error return value
	// is non-nil, which is a common bug pattern in Go.
	checks = append(checks, nilerr.Analyzer)

	// Custom analyzers

	// noexit checks that os.Exit is not called directly in the main function
	// of the main package, enforcing that deferred cleanup always runs.
	checks = append(checks, noexit.Analyzer)
	multichecker.Main(checks...)
}
