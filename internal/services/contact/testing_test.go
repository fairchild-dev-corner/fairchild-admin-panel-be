package services

import (
	"context"
	"errors"
	cc "fairchild-admin-panel-be/internal/constants"
	"fairchild-admin-panel-be/internal/interfaces"
	audit "fairchild-admin-panel-be/internal/models/audit"
	models "fairchild-admin-panel-be/internal/models/contact"
	"testing"
)

type replyRepo struct {
	interfaces.ContactRepositoryInterface
	writes int
}

func (r *replyRepo) FindByID(context.Context, int64) (*models.ContactMessage, error) {
	return &models.ContactMessage{ID: "CM-00001", Email: "member@example.com"}, nil
}
func (r *replyRepo) InsertReply(context.Context, int64, string, string, string, *audit.AuditLog) error {
	r.writes++
	return nil
}

type failedEmail struct{}

func (failedEmail) SendEmail(context.Context, string, string, string) error {
	return errors.New("provider down")
}

func TestContactReplyFailureDoesNotRecord(t *testing.T) {
	r := &replyRepo{}
	s := NewContactService(r, failedEmail{}, nil)
	_, err := s.Reply(context.Background(), "CM-00001", &models.ReplyRequest{Channel: models.ChannelEmail, Body: "Hello"}, audit.Actor{})
	if !errors.Is(err, cc.ErrReplySendFailed) || r.writes != 0 {
		t.Fatalf("err=%v writes=%d", err, r.writes)
	}
}
func TestContactReplyValidation(t *testing.T) {
	r := &replyRepo{}
	s := NewContactService(r, nil, nil)
	for _, tc := range []struct {
		channel, body string
		want          error
	}{
		{"email", " ", cc.ErrReplyEmpty}, {"email", "Hello", cc.ErrReplyEmailUnavailable}, {"sms", "Hello", cc.ErrReplySMSUnavailable}, {"invalid", "Hello", cc.ErrInvalidPayload},
	} {
		if _, err := s.Reply(context.Background(), "1", &models.ReplyRequest{Channel: tc.channel, Body: tc.body}, audit.Actor{}); !errors.Is(err, tc.want) {
			t.Fatal(err)
		}
	}
	if r.writes != 0 {
		t.Fatal("invalid reply recorded")
	}
}
func BenchmarkContact(b *testing.B) {
	s := NewContactService(&replyRepo{}, nil, nil)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := s.Get(context.Background(), "CM-00001"); err != nil {
			b.Fatal(err)
		}
	}
}
