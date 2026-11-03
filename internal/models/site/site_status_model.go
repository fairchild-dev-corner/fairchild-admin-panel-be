package models

import "time"

// SiteStatus matches SiteStatus in the admin panel (src/models/admin.ts).
type SiteStatus struct {
	MaintenanceMode bool      `json:"maintenanceMode"`
	Message         string    `json:"message"`
	UpdatedAt       time.Time `json:"updatedAt"`
	UpdatedBy       string    `json:"updatedBy,omitempty"`
}

// PublicSiteStatus is what the unauthenticated GET /site/status returns to
// fairchild-fe - no staff names.
type PublicSiteStatus struct {
	MaintenanceMode bool      `json:"maintenanceMode"`
	Message         string    `json:"message"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// UpdateSiteStatusRequest is the body of PUT /admin/site-status.
type UpdateSiteStatusRequest struct {
	// Pointer so a missing field is a validation error, not "false".
	MaintenanceMode *bool  `json:"maintenanceMode" validate:"required"`
	Message         string `json:"message" validate:"max=500"`
}
