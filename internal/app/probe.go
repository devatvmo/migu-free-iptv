package app

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxPlaylistDepth = 3

type mediaPlaylist struct {
	URL     string
	Segment string
}

// readPlaylist remains available for isolated parser tests and future
// diagnostics. The default generation path deliberately does not call it.
func readPlaylist(client *http.Client, playlistURL string, depth int) (mediaPlaylist, error) {
	if depth > maxPlaylistDepth {
		return mediaPlaylist{}, fmt.Errorf("M3U8主/子播放列表嵌套超过 %d 层", maxPlaylistDepth)
	}
	req, err := http.NewRequest(http.MethodGet, playlistURL, nil)
	if err != nil {
		return mediaPlaylist{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return mediaPlaylist{}, fmt.Errorf("M3U8请求失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return mediaPlaylist{}, fmt.Errorf("M3U8 HTTP %d", resp.StatusCode)
	}
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	resp.Body.Close()
	if readErr != nil {
		return mediaPlaylist{}, fmt.Errorf("读取播放列表失败: %w", readErr)
	}
	playlist := string(data)
	if !strings.Contains(playlist, "#EXTM3U") {
		return mediaPlaylist{}, fmt.Errorf("响应不是有效 M3U8")
	}

	scanner := bufio.NewScanner(strings.NewReader(playlist))
	var nextURI string
	expectVariant := false
	isMaster := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			isMaster = true
			expectVariant = true
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		if expectVariant {
			nextURI = line
			break
		}
		if !isMaster {
			nextURI = line
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return mediaPlaylist{}, fmt.Errorf("解析M3U8失败: %w", err)
	}
	if nextURI == "" {
		return mediaPlaylist{}, fmt.Errorf("播放列表未找到媒体分片或子播放列表")
	}
	base, err := url.Parse(playlistURL)
	if err != nil {
		return mediaPlaylist{}, fmt.Errorf("播放列表URL解析失败: %w", err)
	}
	ref, err := url.Parse(nextURI)
	if err != nil {
		return mediaPlaylist{}, fmt.Errorf("播放列表URI解析失败: %w", err)
	}
	resolved := base.ResolveReference(ref).String()
	if isMaster {
		return readPlaylist(client, resolved, depth+1)
	}
	return mediaPlaylist{URL: playlistURL, Segment: resolved}, nil
}

// Probe only obtains the original visitor dispatch URL. The CNB runner must
// not fetch the M3U8 or media segments: those requests need to originate from
// the end user's mainland-China TV box network. Publishable means only that
// the URL was obtained and the PID is on the manually reviewed free allowlist;
// it does not mean playback has been verified.
func Probe(ch Channel, freeListed bool) ProbeResult {
	r := ProbeResult{
		Channel:    ch,
		FreeListed: freeListed,
		CheckedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	playURL, err := ResolveVisitor(ch.PID)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	r.Resolved = true
	r.PlayURL = playURL
	r.Publishable = freeListed
	if !freeListed {
		r.Error = "已获取游客调度地址，但尚未加入人工确认的免费频道名单"
	}
	return r
}
