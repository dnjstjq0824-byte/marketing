package store

import (
	"errors"
	"path/filepath"
	"testing"
)

func openInviteStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "invites.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestInviteLifecycle(t *testing.T) {
	st := openInviteStore(t)
	v, err := st.CreateInvite("SOMUN-AAAA-1111", 2, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if v.Code != "SOMUN-AAAA-1111" || v.MaxUses != 2 || v.Used != 0 || v.Revoked {
		t.Fatalf("created = %+v", v)
	}
	got, err := st.GetInvite("SOMUN-AAAA-1111")
	if err != nil || got.ID != v.ID {
		t.Fatalf("get = %+v, %v", got, err)
	}
	if err := st.ConsumeInvite("SOMUN-AAAA-1111"); err != nil {
		t.Fatal(err)
	}
	if err := st.ConsumeInvite("SOMUN-AAAA-1111"); err != nil {
		t.Fatal(err)
	}
	if err := st.ConsumeInvite("SOMUN-AAAA-1111"); !errors.Is(err, ErrInviteSpent) {
		t.Fatalf("third consume = %v, want spent", err)
	}
	list, err := st.ListInvites()
	if err != nil || len(list) != 1 || list[0].Used != 2 {
		t.Fatalf("list = %+v, %v", list, err)
	}
	if err := st.RevokeInvite(v.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.ConsumeInvite("SOMUN-AAAA-1111"); !errors.Is(err, ErrInviteRevoked) {
		t.Fatalf("revoked consume = %v, want revoked", err)
	}
	if err := st.RevokeInvite(999999); !errors.Is(err, ErrNoInvite) {
		t.Fatalf("revoke missing = %v, want not found", err)
	}
	if _, err := st.GetInvite("SOMUN-NOPE-0000"); !errors.Is(err, ErrNoInvite) {
		t.Fatalf("get missing = %v, want not found", err)
	}
}
