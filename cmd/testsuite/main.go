// Command testsuite runs the repository's test procedures without external services.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type step struct {
	Name    string
	Program string
	Args    []string
}
type result struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Duration string `json:"duration"`
	Log      string `json:"log"`
	Error    string `json:"error,omitempty"`
}

func main() { os.Exit(run()) }

func run() int {
	suite := flag.String("suite", "all", "all, unit, performance, security, or fuzz")
	out := flag.String("out", "artifacts/testing", "report directory")
	race := flag.Bool("race", false, "enable race detection for unit tests (requires CGO and a C compiler)")
	benchTime := flag.String("benchtime", "200ms", "time per benchmark")
	fuzzTime := flag.String("fuzztime", "5s", "time per fuzz target")
	timeout := flag.Duration("step-timeout", 15*time.Minute, "maximum duration of each procedure")
	flag.Parse()
	if *timeout <= 0 {
		fmt.Fprintln(os.Stderr, "step-timeout must be positive")
		return 2
	}
	steps, err := procedures(*suite, *out, *race, *benchTime, *fuzzTime)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if _, err := os.Stat("go.mod"); err != nil {
		fmt.Fprintln(os.Stderr, "run from the repository root:", err)
		return 2
	}
	if err := os.MkdirAll(*out, 0755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	var results []result
	failed := false
	for _, s := range steps {
		logPath := filepath.Join(*out, s.Name+".log")
		log, err := os.Create(logPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		fmt.Printf("\nRunning %s: %s %v\n", s.Name, s.Program, s.Args)
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		cmd := exec.CommandContext(ctx, s.Program, s.Args...)
		cmd.Stdout = io.MultiWriter(os.Stdout, log)
		cmd.Stderr = io.MultiWriter(os.Stderr, log)
		started := time.Now()
		err = cmd.Run()
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		cancel()
		closeErr := log.Close()
		if err == nil {
			err = closeErr
		}
		r := result{Name: s.Name, Passed: err == nil, Duration: time.Since(started).Round(time.Millisecond).String(), Log: logPath}
		if err != nil {
			r.Error = err.Error()
			failed = true
			if s.Program == "govulncheck" && errors.Is(err, exec.ErrNotFound) {
				fmt.Fprintln(os.Stderr, "govulncheck must be on PATH; install: go install golang.org/x/vuln/cmd/govulncheck@v1.1.4")
			}
		}
		results = append(results, r)
	}
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if err := os.WriteFile(filepath.Join(*out, "summary.json"), append(data, '\n'), 0644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	fmt.Printf("\nReports: %s\n", *out)
	for _, r := range results {
		fmt.Printf("%s: passed=%v (%s)\n", r.Name, r.Passed, r.Duration)
	}
	if failed {
		return 1
	}
	return 0
}

func procedures(suite, out string, race bool, benchTime, fuzzTime string) ([]step, error) {
	tests := []string{"test", "./...", "-count=1", "-shuffle=on", "-timeout=10m", "-coverpkg=./...", "-covermode=atomic", "-coverprofile=" + filepath.Join(out, "coverage.out")}
	if race {
		tests = append(tests, "-race")
	}
	groups := map[string][]step{
		"unit": {
			{"unit", "go", tests},
			{"coverage", "go", []string{"tool", "cover", "-func=" + filepath.Join(out, "coverage.out")}},
		},
		"performance": {{"performance", "go", []string{"test", "./...", "-run=^$", "-bench=.", "-benchmem", "-benchtime=" + benchTime, "-count=1", "-timeout=10m"}}},
		"security": {
			{"vet", "go", []string{"vet", "./..."}},
			{"security", "go", []string{"test", "./...", "-run=Security", "-count=1", "-timeout=5m"}},
			{"vulnerabilities", "govulncheck", []string{"./..."}},
		},
		"fuzz": {
			{"fuzz-tokens", "go", []string{"test", "./internal/services/auth", "-run=^$", "-fuzz=^FuzzParseAccessToken$", "-fuzztime=" + fuzzTime, "-parallel=2", "-timeout=5m"}},
			{"fuzz-mobile", "go", []string{"test", "./internal/services/messaging", "-run=^$", "-fuzz=^FuzzNormalizeMobileNumber$", "-fuzztime=" + fuzzTime, "-parallel=2", "-timeout=5m"}},
		},
	}
	if suite == "all" {
		var all []step
		for _, name := range []string{"unit", "performance", "security", "fuzz"} {
			all = append(all, groups[name]...)
		}
		return all, nil
	}
	if steps, ok := groups[suite]; ok {
		return steps, nil
	}
	return nil, fmt.Errorf("unknown suite %q", suite)
}
