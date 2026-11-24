package app

import (\n\t"net/http"\n\t"net/http/httptest"
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


func TestResolveDispatchURLFollowsRedirect(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/cdn/live.m3u8", http.StatusFound)
	})
	mux.HandleFunc("/cdn/live.m3u8", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	got, err := resolveDispatchURL(server.URL+"/start", server.Client(), 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != server.URL+"/cdn/live.m3u8" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveDispatchURLRetriesBofangRedirect(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Location", "http://bofang.invalid/dispatch")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	_, err := resolveDispatchURL(server.URL, server.Client(), 4, 0)
	if err == nil {
		t.Fatal("expected bounded retry error")
	}
	if calls != 4 {
		t.Fatalf("expected 4 attempts, got %d", calls)
	}
}
