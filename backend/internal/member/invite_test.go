package member

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func postJSON(t *testing.T, h *Handler, fn func(w http.ResponseWriter, r *http.Request), target, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", target, strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:5000"
	fn(rec, req)
	return rec
}

func TestSignupRequiresInvite(t *testing.T) {
	svc := newTestService(t)
	h := NewHandler(svc)
	rec := postJSON(t, h, h.Signup, "/api/promo/signup",
		`{"email":"n@e.com","password":"password-1","nickname":"n"}`)
	if rec.Code != 403 {
		t.Fatalf("no code = %d, want 403", rec.Code)
	}
	rec = postJSON(t, h, h.Signup, "/api/promo/signup",
		`{"email":"n@e.com","password":"password-1","nickname":"n","inviteCode":"SOMUN-NOPE-0000"}`)
	if rec.Code != 403 {
		t.Fatalf("bad code = %d, want 403", rec.Code)
	}
}

func TestSignupConsumesInvite(t *testing.T) {
	svc := newTestService(t)
	h := NewHandler(svc)
	minted, err := svc.MintInvites(1, 1, "admin")
	if err != nil {
		t.Fatal(err)
	}
	code := minted[0].Code
	rec := postJSON(t, h, h.Signup, "/api/promo/signup",
		`{"email":"a@e.com","password":"password-1","nickname":"n","inviteCode":"`+code+`"}`)
	if rec.Code != 201 {
		t.Fatalf("first use = %d (%s), want 201", rec.Code, rec.Body.String())
	}
	rec = postJSON(t, h, h.Signup, "/api/promo/signup",
		`{"email":"b@e.com","password":"password-1","nickname":"n2","inviteCode":"`+code+`"}`)
	if rec.Code != 403 {
		t.Fatalf("reuse = %d, want 403", rec.Code)
	}
}

func TestMemberLoginRateLimited(t *testing.T) {
	svc := newTestService(t)
	h := NewHandler(svc)
	for i := 0; i < 5; i++ {
		rec := postJSON(t, h, h.Login, "/api/promo/login",
			`{"email":"ghost@e.com","password":"wrong-000"}`)
		if rec.Code != 401 {
			t.Fatalf("attempt %d = %d, want 401", i, rec.Code)
		}
	}
	rec := postJSON(t, h, h.Login, "/api/promo/login",
		`{"email":"ghost@e.com","password":"wrong-000"}`)
	if rec.Code != 429 {
		t.Fatalf("6th attempt = %d, want 429", rec.Code)
	}
}

func TestInviteAdminRoundTrip(t *testing.T) {
	svc := newTestService(t)
	h := NewHandler(svc)
	rec := postJSON(t, h, h.MintInvites, "/api/promo/admin/invites", `{"count":2,"maxUses":3}`)
	if rec.Code != 201 {
		t.Fatalf("mint = %d (%s), want 201", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "SOMUN-") {
		t.Fatalf("mint body missing code: %s", rec.Body.String())
	}
	lrec := httptest.NewRecorder()
	h.ListInvites(lrec, httptest.NewRequest("GET", "/api/promo/admin/invites", nil))
	if lrec.Code != 200 || !strings.Contains(lrec.Body.String(), "SOMUN-") {
		t.Fatalf("list = %d (%s)", lrec.Code, lrec.Body.String())
	}
	list, err := svc.ListInvites()
	if err != nil || len(list) != 2 {
		t.Fatalf("service list = %d, %v", len(list), err)
	}
	rrec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/promo/admin/invites/1/revoke", nil)
	req.SetPathValue("id", "1")
	h.RevokeInvite(rrec, req)
	if rrec.Code != 200 {
		t.Fatalf("revoke = %d (%s), want 200", rrec.Code, rrec.Body.String())
	}
	if err := svc.CheckInvite(list[0].Code); err != nil {
		t.Fatalf("newest code should stay valid: %v", err)
	}
	if err := svc.CheckInvite(list[1].Code); err == nil {
		t.Fatalf("revoked code still valid")
	}
}
