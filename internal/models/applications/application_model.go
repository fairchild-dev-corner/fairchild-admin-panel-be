package models

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// LoanProducts mirrors LoanProduct in the admin panel (src/models/admin.ts).
var LoanProducts = []string{
	"Petty Cash",
	"Hospital Emergency",
	"Socialized Multi-Purpose Loan",
	"Secured Loan",
	"CASE",
}

// LoanApplication matches LoanApplication in the admin panel.
type LoanApplication struct {
	ID          string     `json:"id"`
	MemberID    string     `json:"memberId"`
	MemberName  string     `json:"memberName"`
	Product     string     `json:"product"`
	Amount      float64    `json:"amount"`
	TermMonths  int        `json:"termMonths"`
	Purpose     string     `json:"purpose"`
	Status      string     `json:"status"`
	SubmittedAt time.Time  `json:"submittedAt"`
	ReviewedAt  *time.Time `json:"reviewedAt,omitempty"`
	ReviewedBy  string     `json:"reviewedBy,omitempty"`
	ReviewNote  string     `json:"reviewNote,omitempty"`
}

// MembershipApplication matches MembershipApplication in the admin panel.
// Backed by fairchild_be's membership_applications table plus our
// membership_application_reviews side table.
type MembershipApplication struct {
	ID             string     `json:"id"`
	MembershipType string     `json:"membershipType"`
	FirstName      string     `json:"firstName"`
	LastName       string     `json:"lastName"`
	MiddleName     string     `json:"middleName,omitempty"`
	Suffix         string     `json:"suffix,omitempty"`
	Email          string     `json:"email,omitempty"`
	MobileNumber   string     `json:"mobileNumber,omitempty"`
	DateOfBirth    string     `json:"dateOfBirth"`
	Gender         string     `json:"gender,omitempty"`
	SourceOfIncome string     `json:"sourceOfIncome,omitempty"`
	TypeOfWork     string     `json:"typeOfWork,omitempty"`
	Status         string     `json:"status"`
	SubmittedAt    time.Time  `json:"submittedAt"`
	ReviewedAt     *time.Time `json:"reviewedAt,omitempty"`
	ReviewedBy     string     `json:"reviewedBy,omitempty"`
	ReviewNote     string     `json:"reviewNote,omitempty"`
}

// NewMembershipApplicationRequest is the body of POST
// /admin/membership-applications: a request staff enter on an applicant's
// behalf (NewMembershipApplication in the panel). Option values match
// fairchild-fe's public form.
type NewMembershipApplicationRequest struct {
	MembershipType string `json:"membershipType" validate:"required,oneof=coop laboratory"`
	FirstName      string `json:"firstName" validate:"required,max=100"`
	LastName       string `json:"lastName" validate:"required,max=100"`
	MiddleName     string `json:"middleName" validate:"max=100"`
	Suffix         string `json:"suffix" validate:"max=20"`
	DateOfBirth    string `json:"dateOfBirth" validate:"required,datetime=2006-01-02"`
	Email          string `json:"email" validate:"omitempty,email,max=255"`
	MobileNumber   string `json:"mobileNumber" validate:"required,max=20"`
	Gender         string `json:"gender" validate:"required,oneof=female male prefer-not-to-say"`
	SourceOfIncome string `json:"sourceOfIncome" validate:"omitempty,oneof=salary business remittance pension other"`
	TypeOfWork     string `json:"typeOfWork" validate:"max=100"`
}

// ReviewDecisionRequest is the body of PUT /admin/*-applications/:id/review.
type ReviewDecisionRequest struct {
	Status string `json:"status" validate:"required,oneof=approved rejected"`
	Note   string `json:"note" validate:"max=500"`
}

// FormatID renders a numeric row id as the panel's display id, e.g.
// FormatID("LA-", 1040) == "LA-01040".
func FormatID(prefix string, id int64) string {
	return fmt.Sprintf("%s%05d", prefix, id)
}

// ParseID accepts either the display id ("LA-01040") or the bare number
// ("1040") and returns the row id. ok is false for anything else.
func ParseID(prefix, raw string) (int64, bool) {
	raw = strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(raw)), prefix)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}
