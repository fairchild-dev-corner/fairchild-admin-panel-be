package cc

import "errors"

const (
	UNINITIALIZED_PANIC_STATE = "[panic state] setup/un-initialized"

	// Auth
	INVALID_CREDENTIALS        = "Invalid username or password"
	ACCOUNT_SUSPENDED          = "This account has been suspended. Please contact an administrator"
	ACCOUNT_TEMPORARILY_LOCKED = "Too many failed login attempts, please try again later"
	INVALID_REFRESH_TOKEN      = "Invalid or expired refresh token"
	MISSING_BEARER_TOKEN       = "missing bearer token"
	INVALID_ACCESS_TOKEN       = "invalid or expired access token"
	FORBIDDEN                  = "You do not have permission to perform this action"
	USERNAME_ALREADY_EXISTS    = "Username is already taken"
	EMAIL_ALREADY_EXISTS       = "Email is already used by another panel account"

	// Applications
	APPLICATION_NOT_FOUND        = "Application not found"
	APPLICATION_ALREADY_REVIEWED = "Application has already been reviewed"
	INVALID_STATUS_FILTER        = "status must be one of: pending, approved, rejected"

	// Members / manual membership requests
	MEMBER_NOT_FOUND          = "Member not found"
	LEDGER_MEMBER_READ_ONLY   = "This member's record is managed in the branch's accounting system and can't be changed here"
	PORTAL_MEMBER_NOT_FOUND   = "Portal account not found"
	PORTAL_EMAIL_TAKEN        = "That email address is already used by another portal account"
	PORTAL_ACCOUNT_EXISTS     = "This member number already has a portal account"
	NOT_ON_LEDGER             = "No current member with that member number on that branch's ledger"
	INVALID_PORTAL_FILTER     = "status must be one of: active, pending_review, inactive, suspended"
	INVALID_MEMBER_FILTER     = "category must be one of: regular, associate, kiddie, aflatoun; branches: main, knowles, tiic"
	DATE_OF_BIRTH_IN_FUTURE   = "Date of birth can't be in the future"
	JOINED_AT_IN_FUTURE       = "Membership date can't be in the future"
	INVALID_MOBILE_NUMBER     = "Enter a valid PH mobile number, e.g. 0908 112 2689"
	MOBILE_NUMBER_REQUIRED    = "Mobile number is required"
	GUARDIAN_REQUIRED         = "Guardian's full name and mobile number are required for kiddie and aflatoun members"
	COOP_APPLICANT_INCOMPLETE = "Coop applicants need an email address, source of income and type of work"

	// Generic
	INVALID_PAYLOAD = "invalid payload"
)

// ErrInvalidCredentials covers both an unknown username and a wrong
// password - collapsed on purpose so the response can't be used to probe
// which usernames exist.
var ErrInvalidCredentials = errors.New(INVALID_CREDENTIALS)

// ErrAccountSuspended is returned only after the password has been verified,
// so it doesn't leak account existence to someone guessing usernames.
var ErrAccountSuspended = errors.New(ACCOUNT_SUSPENDED)

// ErrAccountTemporarilyLocked is returned when the username or source IP has
// too many recent failed logins (see AuthService.Login).
var ErrAccountTemporarilyLocked = errors.New(ACCOUNT_TEMPORARILY_LOCKED)

// ErrInvalidRefreshToken covers missing, unknown, revoked, expired and
// already-rotated refresh tokens, and refresh for a no-longer-active user.
var ErrInvalidRefreshToken = errors.New(INVALID_REFRESH_TOKEN)

// ErrInvalidAccessToken is returned by RequirePanelAuth for a bad/expired
// token or one whose user is no longer active.
var ErrInvalidAccessToken = errors.New(INVALID_ACCESS_TOKEN)

var ErrUsernameAlreadyExists = errors.New(USERNAME_ALREADY_EXISTS)
var ErrEmailAlreadyExists = errors.New(EMAIL_ALREADY_EXISTS)

// ErrApplicationNotFound also covers a malformed id, so ids can't be probed.
var ErrApplicationNotFound = errors.New(APPLICATION_NOT_FOUND)

// ErrApplicationAlreadyReviewed is returned when a review races another
// reviewer (or targets an application that's no longer pending).
var ErrApplicationAlreadyReviewed = errors.New(APPLICATION_ALREADY_REVIEWED)

// ErrMemberNotFound also covers a malformed or soft-deleted member id.
var ErrMemberNotFound = errors.New(MEMBER_NOT_FOUND)

// ErrPortalMemberNotFound also covers a malformed or removed account id.
var ErrPortalMemberNotFound = errors.New(PORTAL_MEMBER_NOT_FOUND)

// ErrPortalEmailTaken is a contact update or new account hitting
// users.uq_users_email.
var ErrPortalEmailTaken = errors.New(PORTAL_EMAIL_TAKEN)

// ErrPortalAccountExists: users.member_id is unique - one account per member.
var ErrPortalAccountExists = errors.New(PORTAL_ACCOUNT_EXISTS)

// ErrNotOnLedger: an account can only be added for a current ledger member.
var ErrNotOnLedger = errors.New(NOT_ON_LEDGER)

// ErrLedgerMemberReadOnly is returned for an update or delete of a branch
// ledger member - those records belong to the accounting system.
var ErrLedgerMemberReadOnly = errors.New(LEDGER_MEMBER_READ_ONLY)

// Validation failures beyond what struct tags can express. Handlers answer
// all of them with 400 and the message as-is.
var (
	ErrInvalidPayload          = errors.New(INVALID_PAYLOAD)
	ErrDateOfBirthInFuture     = errors.New(DATE_OF_BIRTH_IN_FUTURE)
	ErrJoinedAtInFuture        = errors.New(JOINED_AT_IN_FUTURE)
	ErrInvalidMobileNumber     = errors.New(INVALID_MOBILE_NUMBER)
	ErrMobileNumberRequired    = errors.New(MOBILE_NUMBER_REQUIRED)
	ErrGuardianRequired        = errors.New(GUARDIAN_REQUIRED)
	ErrCoopApplicantIncomplete = errors.New(COOP_APPLICANT_INCOMPLETE)
)
