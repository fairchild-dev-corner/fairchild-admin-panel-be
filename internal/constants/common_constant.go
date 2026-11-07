package cc

const (
	// status
	SUCCESS = "success"

	// Panel roles (users_panel.role), highest first. The hierarchy is in
	// internal/models/users (role_model.go).
	ROLE_SUPER_ADMIN = "super_admin"
	ROLE_ADMIN       = "admin"
	ROLE_MANAGER     = "manager"
	ROLE_SUPERVISOR  = "supervisor"
	ROLE_STAFF       = "staff"

	// Panel account status (users_panel.status)
	USER_STATUS_ACTIVE    = "active"
	USER_STATUS_SUSPENDED = "suspended"
	USER_STATUS_DELETED   = "deleted"

	// Application review status (loan_applications / membership_applications)
	APPLICATION_STATUS_PENDING  = "pending"
	APPLICATION_STATUS_APPROVED = "approved"
	APPLICATION_STATUS_REJECTED = "rejected"

	// FILTER_ALL is the status (or category) a list is asked for while the
	// admin searches: matches from every tab.
	FILTER_ALL = "all"

	// Membership types (membership_applications / admin_members)
	MEMBERSHIP_TYPE_COOP = "coop"

	// Display id prefixes - the panel shows "LA-01040" / "MA-00560" / "FC20310".
	LOAN_APPLICATION_ID_PREFIX       = "LA-"
	MEMBERSHIP_APPLICATION_ID_PREFIX = "MA-"
	MEMBER_ID_PREFIX                 = "FC"
	PORTAL_MEMBER_ID_PREFIX          = "PU-"
	ACCOUNT_CLAIM_ID_PREFIX          = "CR-"
	LOCKED_ACCOUNT_ID_PREFIX         = "LK-"
	MESSAGE_BLAST_ID_PREFIX          = "MB-"
	CONTACT_MESSAGE_ID_PREFIX        = "CM-"
	CONTACT_REPLY_ID_PREFIX          = "RP-"

	// JWT issuer/audience. Every access token is checked for both, so a token
	// minted by fairchild_be (members) can never authenticate here even if
	// the secrets were ever misconfigured to match.
	JWT_ISSUER   = "fairchild-admin-panel-be"
	JWT_AUDIENCE = "fairchild-admin-panel"

	// REFRESH_TOKEN_COOKIE_NAME is deliberately different from fairchild_be's
	// "refresh_token": cookies aren't port-scoped, so on localhost (and on a
	// shared parent domain) both services' cookies land in the same jar.
	REFRESH_TOKEN_COOKIE_NAME = "admin_refresh_token"
	REFRESH_TOKEN_COOKIE_PATH = "/api/v1/auth"

	// Audit log targets
	AUDIT_TARGET_SITE = "Site"
)
