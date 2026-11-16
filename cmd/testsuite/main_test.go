package main

import (
	"testing"
)

func TestAllProceduresIncludeSecurityAndFuzz(t *testing.T) {
	steps, err := procedures("all", "reports", true, "1x", "1s")
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, s := range steps {
		found[s.Name] = true
	}
	for _, name := range []string{"unit", "coverage", "performance", "vet", "security", "vulnerabilities", "fuzz-tokens", "fuzz-mobile"} {
		if !found[name] {
			t.Errorf("missing %s", name)
		}
	}
	hasRace := false
	for _, arg := range steps[0].Args {
		if arg == "-race" {
			hasRace = true
		}
	}
	if !hasRace {
		t.Fatal("race option ignored")
	}
	if _, err := procedures("typo", "reports", false, "1x", "1s"); err == nil {
		t.Fatal("invalid suite accepted")
	}
}
