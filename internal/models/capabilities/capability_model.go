package models

import (
	cc "fairchild-admin-panel-be/internal/constants"
	users "fairchild-admin-panel-be/internal/models/users"
)

// Capability keys. A page's ".view" key lets an account open the page and
// read its data; the others let it take that page's actions. An action also
// needs its page's view key.
const (
	LoansView   = "loans.view"
	LoansReview = "loans.review"

	MembershipsView   = "memberships.view"
	MembershipsAdd    = "memberships.add"
	MembershipsReview = "memberships.review"

	MembersView   = "members.view"
	MembersEdit   = "members.edit"
	MembersDelete = "members.delete"

	PortalView   = "portal.view"
	PortalManage = "portal.manage"

	ClaimsView   = "claims.view"
	ClaimsReview = "claims.review"

	LocksView   = "locks.view"
	LocksUnlock = "locks.unlock"

	ContactView  = "contact.view"
	ContactReply = "contact.reply"

	BlastsView = "blasts.view"
	BlastsSend = "blasts.send"

	ElectionControllerView = "election.controller.view"
	ElectionControl        = "election.control"
	ElectionMembersView    = "election.members.view"
	MasterListView         = "election.masterlist.view"
	MasterListManage       = "election.masterlist.manage"

	MaintenanceView   = "maintenance.view"
	MaintenanceManage = "maintenance.manage"
	MetricsView       = "metrics.view"
	AuditView         = "audit.view"
)

// Capability is one switch on the Capabilities page. Section and Page match
// the panel's sidebar (src/components/layout/navItems.ts). PresetFrom is the
// lowest role whose preset includes it (so every role above it too); empty
// when no preset does and an admin must assign it by hand.
type Capability struct {
	Key         string `json:"key"`
	Section     string `json:"section"`
	Page        string `json:"page"`
	Label       string `json:"label"`
	Description string `json:"description"`
	View        bool   `json:"view"`
	PresetFrom  string `json:"presetFrom,omitempty"`
}

const (
	staff      = cc.ROLE_STAFF
	supervisor = cc.ROLE_SUPERVISOR
	manager    = cc.ROLE_MANAGER
	byHand     = ""
)

// Catalog lists every capability in sidebar order. Presets grow with rank:
// staff open the pages and enter records, supervisors also review and
// unlock, managers also send, delete and change portal accounts. Turning
// the election on or off and maintenance mode are never preset.
var Catalog = []Capability{
	{LoansView, "Loans", "Loan Applications", "View page", "See loan applications and their details.", true, staff},
	{LoansReview, "Loans", "Loan Applications", "Approve or reject", "Review pending loan applications.", false, supervisor},

	{MembershipsView, "Membership", "Applications", "View page", "See membership applications and their details.", true, staff},
	{MembershipsAdd, "Membership", "Applications", "Add applications", "Enter a walk-in or phone application.", false, staff},
	{MembershipsReview, "Membership", "Applications", "Approve or reject", "Review pending membership applications.", false, supervisor},

	{MembersView, "Membership", "Members", "View page", "See members, statements of account and client standing.", true, staff},
	{MembersEdit, "Membership", "Members", "Add and edit", "Add members and edit members added from the panel.", false, staff},
	{MembersDelete, "Membership", "Members", "Delete", "Delete members added from the panel.", false, manager},

	{PortalView, "Membership", "Web Portal", "View page", "See member portal accounts.", true, staff},
	{PortalManage, "Membership", "Web Portal", "Manage accounts", "Add portal accounts, change their contact details or status, and remove them.", false, manager},

	{ClaimsView, "Membership", "Claim Accounts", "View page", "See account claim requests.", true, staff},
	{ClaimsReview, "Membership", "Claim Accounts", "Approve, reject and delete", "Review claims, which changes the member's portal email or mobile.", false, supervisor},

	{LocksView, "Membership", "Locked Accounts", "View page", "See members locked out after failed sign-ins.", true, staff},
	{LocksUnlock, "Membership", "Locked Accounts", "Unlock", "Let a locked member sign in again straight away.", false, supervisor},

	{ContactView, "Membership", "Contact Messages", "View page", "Read messages sent from the website's contact form.", true, staff},
	{ContactReply, "Membership", "Contact Messages", "Reply", "Reply to the sender by email or text.", false, supervisor},

	{BlastsView, "Membership", "SMS/Email Blasting", "View page", "See past blasts and their delivery.", true, supervisor},
	{BlastsSend, "Membership", "SMS/Email Blasting", "Send and cancel", "Send blasts and test messages to members, and cancel a running blast.", false, manager},

	{ElectionControllerView, "Election", "Controller", "View page", "See whether the election is on and the voter counts.", true, supervisor},
	{ElectionControl, "Election", "Controller", "Turn on or off, reset", "Turn the election on or off and reset the master list.", false, byHand},
	{ElectionMembersView, "Election", "Members List", "View page", "See which members can vote.", true, staff},
	{MasterListView, "Election", "Generate Master List", "View page", "See the current voters' master list.", true, supervisor},
	{MasterListManage, "Election", "Generate Master List", "Upload", "Upload a new voters' master list.", false, manager},

	{MaintenanceView, "System", "Maintenance", "View page", "See maintenance mode and service health.", true, manager},
	{MaintenanceManage, "System", "Maintenance", "Turn on or off", "Put the member portal into maintenance, which signs every member out.", false, byHand},
	{MetricsView, "System", "Metrics", "View page", "See portal usage metrics.", true, supervisor},
	{AuditView, "System", "Audit Log", "View page", "See every change made from the panel.", true, manager},
}

// Find returns the catalog entry for key.
func Find(key string) (Capability, bool) {
	for _, c := range Catalog {
		if c.Key == key {
			return c, true
		}
	}
	return Capability{}, false
}

// ViewKey is the view capability of key's page (key itself for a view key
// or an unknown key).
func ViewKey(key string) string {
	c, ok := Find(key)
	if !ok || c.View {
		return key
	}
	for _, v := range Catalog {
		if v.View && v.Section == c.Section && v.Page == c.Page {
			return v.Key
		}
	}
	return key
}

// Preset is the capabilities a new account with role starts with, in
// catalog order. Empty for admins (they have everything anyway).
func Preset(role string) []string {
	keys := []string{}
	if !users.HasAssignedCapabilities(role) {
		return keys
	}
	for _, c := range Catalog {
		if c.PresetFrom != byHand && users.Rank(role) >= users.Rank(c.PresetFrom) {
			keys = append(keys, c.Key)
		}
	}
	return keys
}

// CatalogResponse is the body of GET /admin/capabilities.
type CatalogResponse struct {
	Items []Capability `json:"items"`
}

// UserCapabilities is the body of GET and PUT /admin/users/:id/capabilities:
// the account and the capabilities it has, in catalog order.
type UserCapabilities struct {
	User         users.PanelUser `json:"user"`
	Capabilities []string        `json:"capabilities"`
}

// UpdateUserCapabilitiesRequest is the body of PUT
// /admin/users/:id/capabilities: the account's full set. Actions whose
// page's view capability isn't in the set are dropped.
type UpdateUserCapabilitiesRequest struct {
	Capabilities []string `json:"capabilities"`
}

// MyCapabilities is the body of GET /admin/capabilities/mine: what the
// signed-in user may do (every key for an admin).
type MyCapabilities struct {
	Role         string   `json:"role"`
	Capabilities []string `json:"capabilities"`
}
