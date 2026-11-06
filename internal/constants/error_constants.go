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
	INVALID_STATUS_FILTER        = "status must be one of: pending, approved, rejected, all"

	// Members / manual membership requests
	MEMBER_NOT_FOUND         = "Member not found"
	LEDGER_MEMBER_READ_ONLY  = "This member's record is managed in the branch's accounting system and can't be changed here"
	NOT_A_LEDGER_MEMBER      = "Statements of account and client standing are only available for branch ledger members"
	STATEMENT_NOT_FOUND      = "Statement not found"
	PORTAL_MEMBER_NOT_FOUND  = "Portal account not found"
	PORTAL_EMAIL_TAKEN       = "That email address is already used by another portal account"
	PORTAL_ACCOUNT_EXISTS    = "This member number already has a portal account"
	NOT_ON_LEDGER            = "No current member with that member number on that branch's ledger"
	INVALID_PORTAL_FILTER    = "status must be one of: active, pending_review, inactive, suspended, all"
	INVALID_MEMBER_FILTER    = "category must be one of: regular, associate, kiddie, aflatoun, all; branches: main, knowles, tiic"
	CLAIM_NOT_FOUND          = "Account claim not found"
	CLAIM_ALREADY_REVIEWED   = "This claim has already been reviewed"
	CLAIM_NO_PORTAL_ACCOUNT  = "No portal account uses this member ID. Add the member on the Portal page first, then approve the claim"
	CLAIM_INVALID_CONTACT    = "The requested email or mobile number isn't valid. Reject the claim and ask the member to submit it again"
	INVALID_CLAIM_FILTER     = "status must be one of: pending, approved, rejected, all; month is YYYY-MM and year is YYYY, not both"
	CLAIM_STILL_PENDING      = "Approve or reject this claim before deleting it"
	LOCKED_ACCOUNT_NOT_FOUND = "This account isn't locked anymore. It may have been unlocked already or the lock expired"
	INVALID_LOCK_FILTER      = "status must be locked, unlocked or all; month is YYYY-MM and year is YYYY, not both"
	BLAST_NOT_FOUND          = "Blast not found"
	BLAST_NOT_SENDING        = "This blast has already finished or been cancelled"
	BLAST_NO_RECIPIENTS      = "No members match this audience with an email or mobile number for the chosen channel"
	BLAST_EMAIL_UNAVAILABLE  = "Email isn't set up on the server, so it can't send email blasts"
	BLAST_SMS_UNAVAILABLE    = "Texting isn't set up on the server, so it can't send SMS blasts"
	BLAST_TEST_ADDRESS       = "Enter a valid email address and/or PH mobile number to send the test to"
	BLAST_TEST_SEND_FAILED   = "The test message couldn't be sent. Check the server log for the provider's error"
	// Recorded on recipients that were never sent.
	BLAST_SKIPPED_TEST_MODE   = "Test mode: not on BLAST_TEST_RECIPIENTS"
	BLAST_SKIPPED_CANCELLED   = "Blast cancelled before this was sent"
	INVALID_VOTER_FILTER      = "status must be all, eligible or ineligible"
	MASTER_LIST_NO_FILE       = "Choose an Excel (.xlsx) or CSV file to upload"
	PASSWORD_INCORRECT        = "That password is incorrect"
	PASSWORD_TOO_SHORT        = "Use at least 12 characters for your new password"
	PASSWORD_UNCHANGED        = "Choose a password different from your current one"
	ELECTION_OFF              = "The election is turned off. An administrator must turn it on in Election > Controller first"
	DATE_OF_BIRTH_IN_FUTURE   = "Date of birth can't be in the future"
	JOINED_AT_IN_FUTURE       = "Membership date can't be in the future"
	INVALID_MOBILE_NUMBER     = "Enter a valid PH mobile number, e.g. 0908 112 2689"
	MOBILE_NUMBER_REQUIRED    = "Mobile number is required"
	GUARDIAN_REQUIRED         = "Guardian's full name and mobile number are required for kiddie and aflatoun members"
	COOP_APPLICANT_INCOMPLETE = "Coop applicants need an email address, source of income and type of work"

	// Contact messages
	CONTACT_MESSAGE_NOT_FOUND = "Contact message not found"
	INVALID_CONTACT_FILTER    = "status must be one of: unreplied, replied, all"
	REPLY_EMAIL_UNAVAILABLE   = "Email replies aren't set up. Add the SMTP settings to the admin backend's .env"
	REPLY_SMS_UNAVAILABLE     = "Text replies aren't set up. Add the Movider settings to the admin backend's .env"
	REPLY_NO_MOBILE           = "No mobile number to text: the sender's email doesn't match a portal account with a valid mobile number"
	REPLY_INVALID_EMAIL       = "The sender's email address isn't valid, so an email reply can't be sent"
	REPLY_EMPTY               = "Write a reply before sending"
	REPLY_TOO_LONG_FOR_SMS    = "Text replies can be at most 480 characters"
	REPLY_SEND_FAILED         = "The reply couldn't be sent. Please try again in a moment"
	REPLY_SENT_NOT_RECORDED   = "The reply was sent, but couldn't be saved to the message history"

	// Generic
	INVALID_PAYLOAD = "invalid payload"
)

// Contact message replies. Unavailable channel / no mobile / invalid input
// answer 400, a provider failure 424, sent-but-not-recorded 409.
var (
	ErrContactMessageNotFound = errors.New(CONTACT_MESSAGE_NOT_FOUND)
	ErrReplyEmailUnavailable  = errors.New(REPLY_EMAIL_UNAVAILABLE)
	ErrReplySMSUnavailable    = errors.New(REPLY_SMS_UNAVAILABLE)
	ErrReplyNoMobile          = errors.New(REPLY_NO_MOBILE)
	ErrReplyInvalidEmail      = errors.New(REPLY_INVALID_EMAIL)
	ErrReplyEmpty             = errors.New(REPLY_EMPTY)
	ErrReplyTooLongForSMS     = errors.New(REPLY_TOO_LONG_FOR_SMS)
	ErrReplySendFailed        = errors.New(REPLY_SEND_FAILED)
	ErrReplySentNotRecorded   = errors.New(REPLY_SENT_NOT_RECORDED)
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

// ErrPasswordIncorrect: a signed-in user re-entered the wrong password to
// confirm a sensitive action.
var ErrPasswordIncorrect = errors.New(PASSWORD_INCORRECT)

// ErrPasswordTooShort / ErrPasswordUnchanged: a new panel password must be
// at least auth.MinPasswordLength characters and differ from the old one.
var ErrPasswordTooShort = errors.New(PASSWORD_TOO_SHORT)
var ErrPasswordUnchanged = errors.New(PASSWORD_UNCHANGED)

// ErrElectionOff: the master list can only change while the election is on.
var ErrElectionOff = errors.New(ELECTION_OFF)

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

// ErrClaimNotFound also covers a malformed claim id.
var ErrClaimNotFound = errors.New(CLAIM_NOT_FOUND)

// ErrClaimAlreadyReviewed: only pending claims can be approved or rejected.
var ErrClaimAlreadyReviewed = errors.New(CLAIM_ALREADY_REVIEWED)

// ErrClaimNoPortalAccount: approving updates the portal account whose
// member_id matches the claim, so one has to exist.
var ErrClaimNoPortalAccount = errors.New(CLAIM_NO_PORTAL_ACCOUNT)

// ErrClaimInvalidContact: the claim's proposed email or mobile can't be
// stored on a portal account as-is.
var ErrClaimInvalidContact = errors.New(CLAIM_INVALID_CONTACT)

// ErrClaimStillPending: only a reviewed claim can be deleted.
var ErrClaimStillPending = errors.New(CLAIM_STILL_PENDING)

// ErrLockedAccountNotFound covers an unknown or malformed lock id and a lock
// that has since expired or been lifted.
var ErrLockedAccountNotFound = errors.New(LOCKED_ACCOUNT_NOT_FOUND)

// ErrBlastNotFound also covers a malformed blast id.
var ErrBlastNotFound = errors.New(BLAST_NOT_FOUND)

// ErrBlastNotSending: only a blast that's still sending can be cancelled.
var ErrBlastNotSending = errors.New(BLAST_NOT_SENDING)

// ErrBlastNoRecipients: the audience has nobody reachable on the channel.
var ErrBlastNoRecipients = errors.New(BLAST_NO_RECIPIENTS)

// ErrBlastEmailUnavailable / ErrBlastSMSUnavailable: that provider has no
// credentials on this server.
var ErrBlastEmailUnavailable = errors.New(BLAST_EMAIL_UNAVAILABLE)
var ErrBlastSMSUnavailable = errors.New(BLAST_SMS_UNAVAILABLE)

// ErrBlastTestAddress: a test send needs an address for each channel.
var ErrBlastTestAddress = errors.New(BLAST_TEST_ADDRESS)

// ErrBlastTestSendFailed: the provider refused the test message.
var ErrBlastTestSendFailed = errors.New(BLAST_TEST_SEND_FAILED)

// ErrBlastInvalid is matched (via errors.Is) by every BlastInputError - a
// draft or audience the admin needs to fix. Its message says what.
var ErrBlastInvalid = errors.New("invalid blast")

type BlastInputError struct {
	Msg string
}

func (e *BlastInputError) Error() string { return e.Msg }

func (e *BlastInputError) Is(target error) bool { return target == ErrBlastInvalid }

// ErrMasterListInvalid is matched (via errors.Is) by every
// MasterListInputError - an upload the admin needs to fix (wrong file type,
// no "Member ID" column, too big, nobody with a portal account).
var ErrMasterListInvalid = errors.New("invalid master list")

type MasterListInputError struct {
	Msg string
}

func (e *MasterListInputError) Error() string { return e.Msg }

func (e *MasterListInputError) Is(target error) bool { return target == ErrMasterListInvalid }

// ErrLedgerMemberReadOnly is returned for an update or delete of a branch
// ledger member - those records belong to the accounting system.
var ErrLedgerMemberReadOnly = errors.New(LEDGER_MEMBER_READ_ONLY)

// ErrNotLedgerMember is returned when SOA or client standing is asked for a
// member added in the panel: only ledger members have accounts.
var ErrNotLedgerMember = errors.New(NOT_A_LEDGER_MEMBER)

// ErrStatementNotFound also covers a CTRLNO belonging to another member.
var ErrStatementNotFound = errors.New(STATEMENT_NOT_FOUND)

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
