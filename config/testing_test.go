package config

import "testing"

func TestBranchPairValidation(t *testing.T) {
	got, err := parsePairs(" main = main_db,tiic=tiic_db ")
	if err != nil || len(got) != 2 || got[0].Key != "main" || got[1].Value != "tiic_db" {
		t.Fatalf("%+v %v", got, err)
	}
	for _, raw := range []string{"main=a,main=b", "main", "=database", "main="} {
		if _, err := parsePairs(raw); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
}
func TestSecurityProductionRequiresCA(t *testing.T) {
	original := Envs
	defer func() { Envs = original }()
	Envs.MySQLProdCACert = ""
	if _, err := GetMySQLConfig("prod"); err == nil {
		t.Fatal("production allowed no CA")
	}
}
func BenchmarkConfig(b *testing.B) {
	for b.Loop() {
		if _, err := parsePairs("main=main_db,knowles=knowles_db,tiic=tiic_db"); err != nil {
			b.Fatal(err)
		}
	}
}
