package app

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestVisitor720pDDCalcu(t *testing.T) {
	got := visitor720pDDCalcu("0123456789abcdef", "123456789", "20261124093000")
	want := "f0e1vd2ac3xb4aa59687"
	if got != want {
		t.Fatalf("visitor720pDDCalcu() = %q, want %q", got, want)
	}
}

func TestSealVisitorURL(t *testing.T) {
	raw := "https://example.test/live.m3u8?token=abc&puData=0123456789abcdef"
	got, err := sealVisitorURL(raw, "123456789", time.Date(2026, 11, 24, 9, 30, 0, 0, time.FixedZone("CST", 8*60*60)))
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("ddCalcu") != "f0e1vd2ac3xb4aa59687" {
		t.Fatalf("unexpected ddCalcu: %q", q.Get("ddCalcu"))
	}
	if q.Get("sv") != "10004" || q.Get("ct") != "android" {
		t.Fatalf("missing Android URL suffix parameters: %v", q)
	}
	if !strings.Contains(got, "puData=") {
		t.Fatalf("puData was lost: %s", got)
	}
}

func TestSealVisitorURLWithoutPUData(t *testing.T) {
	raw := "https://example.test/live.m3u8?token=abc"
	got, err := sealVisitorURL(raw, "123456789", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got != raw {
		t.Fatalf("URL without puData should remain unchanged: %s", got)
	}
}
