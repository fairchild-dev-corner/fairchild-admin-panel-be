package messaging

import (
	"bytes"
	"embed"
	"html/template"
	"strings"
	"unicode"
)

//go:embed templates/contact_reply.html
var contactReplyFS embed.FS

var contactReplyTmpl = template.Must(template.ParseFS(contactReplyFS, "templates/contact_reply.html"))

// BrandName matches fairchild_be's emails.
const BrandName = "FCCMPC"

// ContactReplySubject is the subject line of an emailed contact reply.
const ContactReplySubject = "Re: Your message to " + BrandName

// ContactReplyData fills templates/contact_reply.html. html/template escapes
// every field, so the admin's reply and the original message are safe to
// insert as-is.
type ContactReplyData struct {
	BrandName  string
	Name       string
	Body       string
	ReceivedAt string
	Original   string
}

func RenderContactReply(data ContactReplyData) (string, error) {
	data.BrandName = BrandName
	var buf bytes.Buffer
	if err := contactReplyTmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

//go:embed templates/account_unlocked.html
var accountUnlockedFS embed.FS

var accountUnlockedTmpl = template.Must(template.ParseFS(accountUnlockedFS, "templates/account_unlocked.html"))

// AccountUnlockedSubject is the subject line of the email sent when an admin
// unlocks a member's portal account.
const AccountUnlockedSubject = "Your " + BrandName + " member portal account has been unlocked"

// AccountUnlockedSMS is the text sent for the same unlock, greeting the
// member by first name when there is one.
func AccountUnlockedSMS(firstName string) string {
	greeting := "Hi"
	if name := FriendlyFirstName(firstName); name != "" {
		greeting += " " + name
	}
	return greeting + ", your account has been unlocked. You can now log in again to the member portal. " +
		"If this wasn't you, email fairchildcoop@fccmpc.org."
}

// FriendlyFirstName keeps a greeting short and friendly: the first given
// name only, capitalised ("JOSEPH GLENN" -> "Joseph").
func FriendlyFirstName(firstName string) string {
	fields := strings.Fields(firstName)
	if len(fields) == 0 {
		return ""
	}
	r := []rune(strings.ToLower(fields[0]))
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// AccountUnlockedData fills templates/account_unlocked.html.
type AccountUnlockedData struct {
	BrandName  string
	Name       string
	MemberNo   string
	UnlockedAt string
}

func RenderAccountUnlocked(data AccountUnlockedData) (string, error) {
	data.BrandName = BrandName
	var buf bytes.Buffer
	if err := accountUnlockedTmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

//go:embed templates/announcement.html
var announcementFS embed.FS

var announcementTmpl = template.Must(template.ParseFS(announcementFS, "templates/announcement.html"))

// AnnouncementData fills templates/announcement.html, the email an SMS/email
// blast sends. html/template escapes every field, so the admin's text is
// safe to insert as-is.
type AnnouncementData struct {
	BrandName string
	Subject   string
	Body      string
}

func RenderAnnouncement(data AnnouncementData) (string, error) {
	data.BrandName = BrandName
	var buf bytes.Buffer
	if err := announcementTmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

//go:embed templates/claim_approved.html
var claimApprovedFS embed.FS

var claimApprovedTmpl = template.Must(template.ParseFS(claimApprovedFS, "templates/claim_approved.html"))

// ClaimApprovedSubject is the subject line of the email sent when an admin
// approves a member's Claim Account request.
const ClaimApprovedSubject = "Your " + BrandName + " account claim has been approved"

// ClaimApprovedSMS is the text sent for the same approval, greeting the
// member by first name when there is one.
func ClaimApprovedSMS(firstName string) string {
	greeting := "Hi"
	if name := FriendlyFirstName(firstName); name != "" {
		greeting += " " + name
	}
	return greeting + ", your " + BrandName + " account claim was approved. This number and your new email are now on your " +
		"member portal account. Use Forgot Password to set your password. If this wasn't you, email fairchildcoop@fccmpc.org."
}

// ClaimApprovedData fills templates/claim_approved.html.
type ClaimApprovedData struct {
	BrandName string
	Name      string
	MemberNo  string
	Email     string
	Mobile    string
}

func RenderClaimApproved(data ClaimApprovedData) (string, error) {
	data.BrandName = BrandName
	var buf bytes.Buffer
	if err := claimApprovedTmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

//go:embed templates/claim_rejected.html
var claimRejectedFS embed.FS

var claimRejectedTmpl = template.Must(template.ParseFS(claimRejectedFS, "templates/claim_rejected.html"))

// ClaimRejectedSubject is the subject line of the email sent when an admin
// rejects a member's Claim Account request.
const ClaimRejectedSubject = "Your " + BrandName + " account claim request was not approved"

// ClaimRejectedSMS is the text sent for the same rejection, with the
// admin's reason when there is one.
func ClaimRejectedSMS(firstName, reason string) string {
	greeting := "Hi"
	if name := FriendlyFirstName(firstName); name != "" {
		greeting += " " + name
	}
	msg := greeting + ", your " + BrandName + " account claim request was not approved."
	if reason = strings.TrimRight(strings.TrimSpace(reason), ".!? "); reason != "" {
		msg += " Reason: " + reason + "."
	}
	return msg + " You may submit a new request or visit your branch. Questions: fairchildcoop@fccmpc.org."
}

// ClaimRejectedData fills templates/claim_rejected.html.
type ClaimRejectedData struct {
	BrandName string
	Name      string
	MemberNo  string
	Reason    string
}

func RenderClaimRejected(data ClaimRejectedData) (string, error) {
	data.BrandName = BrandName
	var buf bytes.Buffer
	if err := claimRejectedTmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
