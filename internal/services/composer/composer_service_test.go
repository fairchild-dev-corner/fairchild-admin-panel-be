package services

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cc "fairchild-admin-panel-be/internal/constants"
	models "fairchild-admin-panel-be/internal/models/blasts"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// fakeOpenAI answers POST /chat/completions with status and a completion
// whose content is draftJSON and finish reason is finish (a refusal when
// finish is "refusal"), and records the last request body.
func fakeOpenAI(t *testing.T, status int, finish, draftJSON string, got *map[string]any) *ComposerService {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if got != nil {
			_ = json.Unmarshal(body, got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status != http.StatusOK {
			_, _ = w.Write([]byte(`{"error":{"message":"slow down","type":"rate_limit_error","param":null,"code":null}}`))
			return
		}
		content, _ := json.Marshal(draftJSON)
		refusal := "null"
		if finish == "refusal" {
			content, refusal, finish = []byte("null"), `"I can't help with that."`, "stop"
		}
		_, _ = w.Write([]byte(`{"id":"chatcmpl-1","object":"chat.completion","created":1,"model":"gpt-4o-mini",` +
			`"choices":[{"index":0,"finish_reason":"` + finish + `","logprobs":null,` +
			`"message":{"role":"assistant","content":` + string(content) + `,"refusal":` + refusal + `}}],` +
			`"usage":{"prompt_tokens":10,"completion_tokens":10,"total_tokens":20}}`))
	}))
	t.Cleanup(srv.Close)

	s := NewComposerService("test-key", "")
	s.client = openai.NewClient(option.WithAPIKey("test-key"), option.WithBaseURL(srv.URL+"/"), option.WithMaxRetries(0))
	return s
}

const fullDraft = `{"subject":"General Assembly on [date]","emailBody":"Hi {name},\n\nOur General Assembly is on [date].","smsBody":"Hi {name}, our General Assembly is on [date]. - FCCMPC"}`

func TestDraftSendsTheRequestAndReturnsTheDraft(t *testing.T) {
	var body map[string]any
	s := fakeOpenAI(t, http.StatusOK, "stop", fullDraft, &body)

	draft, err := s.Draft(context.Background(), &models.ComposeBlastRequest{
		Channel: models.ChannelBoth, Theme: "Annual General Assembly", Details: "Bring your member ID", Tone: models.ToneFormal,
	})
	if err != nil {
		t.Fatal(err)
	}
	if draft.Subject == "" || !strings.Contains(draft.EmailBody, "{name}") || draft.SMSBody == "" {
		t.Errorf("draft = %+v", draft)
	}

	if body["model"] != DefaultModel {
		t.Errorf("model %v, want %s", body["model"], DefaultModel)
	}
	format, _ := body["response_format"].(map[string]any)
	schema, _ := format["json_schema"].(map[string]any)
	if format["type"] != "json_schema" || schema["strict"] != true {
		t.Errorf("response_format = %v", format)
	}
	prompt, _ := json.Marshal(body["messages"])
	for _, want := range []string{"Annual General Assembly", "Bring your member ID", "formal"} {
		if !strings.Contains(string(prompt), want) {
			t.Errorf("prompt is missing %q: %s", want, prompt)
		}
	}
}

func TestDraftBlanksFieldsTheChannelDoesNotUse(t *testing.T) {
	s := fakeOpenAI(t, http.StatusOK, "stop", fullDraft, nil)

	sms, err := s.Draft(context.Background(), &models.ComposeBlastRequest{Channel: models.ChannelSMS, Theme: "x"})
	if err != nil || sms.Subject != "" || sms.EmailBody != "" || sms.SMSBody == "" {
		t.Errorf("sms draft = %+v, %v", sms, err)
	}
	email, err := s.Draft(context.Background(), &models.ComposeBlastRequest{Channel: models.ChannelEmail, Theme: "x"})
	if err != nil || email.SMSBody != "" || email.Subject == "" {
		t.Errorf("email draft = %+v, %v", email, err)
	}
}

func TestDraftClipsToTheBlastLimits(t *testing.T) {
	long, _ := json.Marshal(map[string]string{
		"subject": strings.Repeat("s", 300), "emailBody": strings.Repeat("e", 6000), "smsBody": strings.Repeat("m", 600),
	})
	s := fakeOpenAI(t, http.StatusOK, "stop", string(long), nil)
	draft, err := s.Draft(context.Background(), &models.ComposeBlastRequest{Channel: models.ChannelBoth, Theme: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Subject) != models.MaxSubjectLength || len(draft.EmailBody) != models.MaxEmailBodyLength || len(draft.SMSBody) != models.MaxSMSLength {
		t.Errorf("lengths %d/%d/%d", len(draft.Subject), len(draft.EmailBody), len(draft.SMSBody))
	}
}

func TestDraftErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		stop   string
		text   string
		theme  string
		want   error
	}{
		{"no theme", http.StatusOK, "stop", fullDraft, "  ", cc.ErrComposeThemeRequired},
		{"refusal", http.StatusOK, "refusal", "", "x", cc.ErrComposeDeclined},
		{"declined with empty fields", http.StatusOK, "stop", `{"subject":"","emailBody":"","smsBody":""}`, "x", cc.ErrComposeDeclined},
		{"cut off", http.StatusOK, "length", `{"subject":"`, "x", cc.ErrComposeFailed},
		{"not JSON", http.StatusOK, "stop", "Sure! Here is a draft", "x", cc.ErrComposeFailed},
		{"rate limited", http.StatusTooManyRequests, "", "", "x", cc.ErrComposeBusy},
		{"overloaded", http.StatusServiceUnavailable, "", "", "x", cc.ErrComposeBusy},
		{"bad key", http.StatusUnauthorized, "", "", "x", cc.ErrComposeUnavailable},
		{"server error", http.StatusInternalServerError, "", "", "x", cc.ErrComposeFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := fakeOpenAI(t, tt.status, tt.stop, tt.text, nil)
			_, err := s.Draft(context.Background(), &models.ComposeBlastRequest{Channel: models.ChannelBoth, Theme: tt.theme})
			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}
