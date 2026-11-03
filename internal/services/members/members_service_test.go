package services

import (
	"errors"
	"testing"
	"time"

	cc "fairchild-admin-panel-be/internal/constants"
	models "fairchild-admin-panel-be/internal/models/members"
)

func validRegular() *models.MemberRequest {
	return &models.MemberRequest{
		MembershipType: "coop",
		Category:       "regular",
		Branch:         "main",
		FirstName:      " Maria ",
		LastName:       "Santos",
		DateOfBirth:    "1988-03-14",
		Gender:         "female",
		MobileNumber:   "0917 123 4501",
		Status:         "active",
		JoinedAt:       "2019-06-03",
	}
}

func TestNormalizeMember(t *testing.T) {
	tomorrow := time.Now().AddDate(0, 0, 2).Format(time.DateOnly)

	tests := []struct {
		name   string
		modify func(r *models.MemberRequest)
		want   error
	}{
		{"valid regular", func(r *models.MemberRequest) {}, nil},
		{"blank name", func(r *models.MemberRequest) { r.LastName = "  " }, cc.ErrInvalidPayload},
		{"future birth date", func(r *models.MemberRequest) { r.DateOfBirth = tomorrow }, cc.ErrDateOfBirthInFuture},
		{"future joined date", func(r *models.MemberRequest) { r.JoinedAt = tomorrow }, cc.ErrJoinedAtInFuture},
		{"adult without mobile", func(r *models.MemberRequest) { r.MobileNumber = "" }, cc.ErrMobileNumberRequired},
		{"bad mobile", func(r *models.MemberRequest) { r.MobileNumber = "12345" }, cc.ErrInvalidMobileNumber},
		{"kiddie without guardian", func(r *models.MemberRequest) {
			r.Category, r.MobileNumber = "kiddie", ""
		}, cc.ErrGuardianRequired},
		{"kiddie with guardian, no own mobile", func(r *models.MemberRequest) {
			r.Category, r.MobileNumber = "kiddie", ""
			r.GuardianFullName, r.GuardianMobileNumber = "Ana Garcia", "+63 919 123 4503"
		}, nil},
		{"bad guardian mobile", func(r *models.MemberRequest) {
			r.Category = "aflatoun"
			r.GuardianFullName, r.GuardianMobileNumber = "Ana Garcia", "0800"
		}, cc.ErrInvalidMobileNumber},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validRegular()
			tt.modify(req)
			if err := normalizeMember(req); !errors.Is(err, tt.want) {
				t.Fatalf("normalizeMember() = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestNormalizeMemberCleansFields(t *testing.T) {
	req := validRegular()
	req.GuardianFullName, req.GuardianMobileNumber = "Stale Guardian", "09170000000"

	if err := normalizeMember(req); err != nil {
		t.Fatal(err)
	}
	if req.FirstName != "Maria" {
		t.Errorf("FirstName = %q, want trimmed", req.FirstName)
	}
	if req.MobileNumber != "09171234501" {
		t.Errorf("MobileNumber = %q, want spaces removed", req.MobileNumber)
	}
	if req.GuardianFullName != "" || req.GuardianMobileNumber != "" {
		t.Error("guardian fields should be cleared for an adult category")
	}
}
