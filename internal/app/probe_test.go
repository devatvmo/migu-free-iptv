package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReadPlaylistResolvesMasterAndRelativeMediaURI(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/master.m3u8", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1200000\nlevels/720/index.m3u8\n")
	})
	mux.HandleFunc("/levels/720/index.m3u8", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "#EXTM3U\n#EXT-X-TARGETDURATION:6\n#EXTINF:6.0,\n../../segments/0001.ts\n")
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	got, err := readPlaylist(server.Client(), server.URL+"/master.m3u8", 0)
	if err != nil {
		t.Fatal(err)
	}
	want := server.URL + "/segments/0001.ts"
	if got.Segment != want {
		t.Fatalf("segment = %q, want %q", got.Segment, want)
	}
	if got.URL != server.URL+"/levels/720/index.m3u8" {
		t.Fatalf("media playlist URL = %q", got.URL)
	}
}

func TestReadPlaylistRejectsEmptyPlaylist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "#EXTM3U\n#EXT-X-ENDLIST\n")
	}))
	defer server.Close()

	if _, err := readPlaylist(server.Client(), server.URL, 0); err == nil {
		t.Fatal("expected missing media segment error")
	}
}

func TestReadPlaylistRejectsExcessiveNesting(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1000\n/next.m3u8\n")
	}))
	defer server.Close()

	if _, err := readPlaylist(server.Client(), server.URL, maxPlaylistDepth+1); err == nil {
		t.Fatal("expected nesting depth error")
	}
}
