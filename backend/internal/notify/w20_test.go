package notify

import (
	"testing"
)

// W20-4: low disk space files exactly one live error alert.
func TestW20DiskLowAlert(t *testing.T) {
	st := openNotifyStore(t)
	svc := NewService(st)
	made, err := svc.DiskLow(9.4)
	if err != nil || !made {
		t.Fatalf("first = %v, %v; want true, nil", made, err)
	}
	made, err = svc.DiskLow(9.1)
	if err != nil || made {
		t.Fatalf("repeat = %v, %v; want false, nil", made, err)
	}
	items, unread, err := svc.List(true, 10, 0)
	if err != nil || unread != 1 || len(items) != 1 {
		t.Fatalf("list = %d items unread %d, %v; want 1/1", len(items), unread, err)
	}
	if items[0].Kind != KindDiskLow || items[0].Severity != "error" {
		t.Fatalf("row = %+v", items[0])
	}
}
