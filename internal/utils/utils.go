package utils

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
)

var Validate = validator.New()

// ManilaTZ is the cooperative's local time, used for "this month" and
// per-month metric buckets. Falls back to a fixed UTC+8 zone when the host
// has no tzdata (e.g. a bare Alpine image).
var ManilaTZ = func() *time.Location {
	if loc, err := time.LoadLocation("Asia/Manila"); err == nil {
		return loc
	}
	return time.FixedZone("PHT", 8*60*60)
}()

func MustStringSHA256(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// NullIfEmpty stores an empty optional string as NULL.
func NullIfEmpty(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// EscapeLike makes user input match literally inside a LIKE pattern.
func EscapeLike(s string) string { return likeEscaper.Replace(s) }

// Truncate cuts s to at most n bytes - used for client-supplied values (user
// agent) written into fixed-width columns.
func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
