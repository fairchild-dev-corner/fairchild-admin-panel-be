package models

import "time"

// Actor is the authenticated panel user performing a mutation, built from
// RequirePanelAuth's context (middlewares.ActorFromContext) - never from the
// request body.
type Actor struct {
	UserPanelID int64
	Name        string
	IP          string
}

// Log builds the audit row for an action this actor performed on target.
func (a Actor) Log(action, target string) *AuditLog {
	return &AuditLog{
		UserPanelID: a.UserPanelID,
		ActorName:   a.Name,
		Action:      action,
		Target:      target,
		IPAddress:   a.IP,
	}
}

// AuditLog is one admin_audit_logs row to be written.
type AuditLog struct {
	UserPanelID int64
	ActorName   string
	Action      string
	Target      string
	IPAddress   string
}

// AuditLogEntry matches AuditLogEntry in the admin panel (src/models/admin.ts).
type AuditLogEntry struct {
	ID     string    `json:"id"`
	Actor  string    `json:"actor"`
	Action string    `json:"action"`
	Target string    `json:"target"`
	At     time.Time `json:"at"`
}
