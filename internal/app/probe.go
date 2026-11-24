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

func Probe(ch Channel, freeListed bool) ProbeResult {
	r := ProbeResult{Channel: ch, FreeListed: freeListed, CheckedAt: time.Now().UTC().Format(time.RFC3339)}
	playURL, err := ResolveVisitor(ch.PID)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	r.Resolved, r.PlayURL = true, playURL

	client := &http.Client{Timeout: 12 * time.Second}
	media, err := readPlaylist(client, playURL, 0)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	r.Playlist = true

	sreq, err := http.NewRequest(http.MethodGet, media.Segment, nil)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	sreq.Header.Set("Range", "bytes=0-1023")
	sreq.Header.Set("User-Agent", "Mozilla/5.0")
	sresp, err := client.Do(sreq)
	if err != nil {
		r.Error = "媒体分片抽样请求失败: " + err.Error()
		return r
	}
	defer sresp.Body.Close()
	if sresp.StatusCode != http.StatusPartialContent && sresp.StatusCode != http.StatusOK {
		r.Error = fmt.Sprintf("媒体分片抽样 HTTP %d", sresp.StatusCode)
		return r
	}
	sample, err := io.ReadAll(io.LimitReader(sresp.Body, 1024))
	if err != nil || len(sample) == 0 {
		r.Error = "媒体分片抽样内容为空"
		return r
	}
	ct := strings.ToLower(sresp.Header.Get("Content-Type"))
	if ct != "" && !strings.Contains(ct, "video/mp2t") && !strings.Contains(ct, "video/mp4") && !strings.Contains(ct, "octet-stream") {
		r.Error = "媒体分片 Content-Type 异常: " + ct
		return r
	}
	r.Segment = true
	r.Passed = r.Resolved && r.Playlist && r.Segment && freeListed
	if !freeListed {
		r.Error = "技术抽样通过，但尚未加入人工确认的免费频道名单"
	}
	return r
}
