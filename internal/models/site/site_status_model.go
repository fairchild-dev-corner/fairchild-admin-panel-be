package models

import "time"

// MaxMaintenanceMinutes is the longest window maintenance can be set to lift
// on its own after (7 days).
const MaxMaintenanceMinutes = 7 * 24 * 60

// MaxScheduleAheadDays is how far ahead maintenance can be scheduled.
const MaxScheduleAheadDays = 30

// SiteStatus matches SiteStatus in the admin panel (src/models/admin.ts).
// MaintenanceMode is the effective state: false before a scheduled start and
// once EndsAt has passed, even though nobody turned it off.
type SiteStatus struct {
	MaintenanceMode bool `json:"maintenanceMode"`
	// Only while maintenance is scheduled and hasn't started yet; members are
	// warned until then.
	StartsAt  *time.Time `json:"startsAt,omitempty"`
	Message   string     `json:"message"`
	EndsAt    *time.Time `json:"endsAt,omitempty"`
	UpdatedAt time.Time  `json:"updatedAt"`
	UpdatedBy string     `json:"updatedBy,omitempty"`
	// Only on the save that started maintenance: how many member sessions
	// were signed out.
	MembersSignedOut *int64 `json:"membersSignedOut,omitempty"`
}

// Scheduled reports whether maintenance is set to start later.
func (s *SiteStatus) Scheduled() bool {
	return s.StartsAt != nil
}

// PublicSiteStatus is what the unauthenticated GET /site/status returns -
// no staff names.
type PublicSiteStatus struct {
	MaintenanceMode bool       `json:"maintenanceMode"`
	StartsAt        *time.Time `json:"startsAt,omitempty"`
	Message         string     `json:"message"`
	EndsAt          *time.Time `json:"endsAt,omitempty"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

// UpdateSiteStatusRequest is the body of PUT /admin/site-status.
type UpdateSiteStatusRequest struct {
	// Pointer so a missing field is a validation error, not "false".
	MaintenanceMode *bool  `json:"maintenanceMode" validate:"required"`
	Message         string `json:"message" validate:"max=500"`
	// With maintenance on: when it starts. Missing starts it now; a time in
	// the future schedules it (up to MaxScheduleAheadDays ahead), warning
	// members until then.
	StartsAt *time.Time `json:"startsAt"`
	// Lift maintenance on its own this many minutes after it starts. Missing
	// keeps the current end time (none if maintenance was off); 0 clears it,
	// so maintenance stays on until an admin turns it off.
	DurationMinutes *int `json:"durationMinutes" validate:"omitempty,min=0,max=10080"`
}
