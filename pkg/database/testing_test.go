package database

import (
	models "fairchild-admin-panel-be/internal/models/config"
	"testing"
	"time"
)

func TestSecurityDriverConfig(t *testing.T) {
	c := models.NewMySQLConfig("localhost", "3306", "user", "p@ss;word", "test", "dev", "")
	got, err := toDriverConfig(c, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.MultiStatements || !got.ParseTime || got.Loc != time.UTC || got.Passwd != "p@ss;word" || got.Params["time_zone"] != "'+00:00'" {
		t.Fatalf("unsafe driver configuration")
	}
	c.CACert = "invalid PEM"
	if _, err := toDriverConfig(c, false); err == nil {
		t.Fatal("invalid CA accepted")
	}
}
func BenchmarkDatabaseConfig(b *testing.B) {
	c := models.NewMySQLConfig("localhost", "3306", "user", "test", "test", "dev", "")
	b.ReportAllocs()
	for b.Loop() {
		if _, err := toDriverConfig(c, false); err != nil {
			b.Fatal(err)
		}
	}
}
