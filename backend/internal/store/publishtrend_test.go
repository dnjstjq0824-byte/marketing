package store

import (
	"path/filepath"
	"testing"
	"time"
)

// Trend buckets done/failed logs by UTC day, oldest first, zeros filled.
func TestPublishTrend(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "trend.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	p, err := st.CreateProduct("https://shop.test/t", "A", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	today := now.Format(time.RFC3339)
	yesterday := now.AddDate(0, 0, -1).Format(time.RFC3339)
	for i, exec := range []string{today, today, yesterday} {
		c, err := st.CreateContent(&Content{ProductID: p.ID, Channel: "blog"})
		if err != nil {
			t.Fatal(err)
		}
		acc := "acc-today"
		if i == 2 {
			acc = "acc-yday"
		}
		if _, err := st.CreatePublishDone(c.ID, "blog", acc, exec, "ok"); err != nil {
			t.Fatal(err)
		}
	}
	cf, err := st.CreateContent(&Content{ProductID: p.ID, Channel: "blog"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreatePublishLog(cf.ID, "blog", "acc-fail", "failed", "nope"); err != nil {
		t.Fatal(err)
	}
	days, err := st.PublishTrend(3)
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 3 {
		t.Fatalf("len = %d", len(days))
	}
	if days[0].Published != 0 || days[0].Failed != 0 {
		t.Fatalf("day0 = %+v", days[0])
	}
	if days[1].Published != 1 || days[1].Failed != 0 {
		t.Fatalf("day1 = %+v", days[1])
	}
	if days[2].Published != 2 || days[2].Failed != 1 {
		t.Fatalf("day2 = %+v", days[2])
	}
	if !(days[0].Date < days[1].Date && days[1].Date < days[2].Date) {
		t.Fatalf("order = %s %s %s", days[0].Date, days[1].Date, days[2].Date)
	}
}
