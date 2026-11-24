package app

import (
	"bufio"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func Probe(ch Channel, freeListed bool) ProbeResult {
	r := ProbeResult{Channel:ch, FreeListed:freeListed, CheckedAt:time.Now().UTC().Format(time.RFC3339)}
	playURL, err := ResolveVisitor(ch.PID)
	if err != nil { r.Error = err.Error(); return r }
	r.Resolved = true
	r.PlayURL = playURL
	client := &http.Client{Timeout:12*time.Second}
	req, _ := http.NewRequest(http.MethodGet, playURL, nil)
	req.Header.Set("User-Agent","Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil { r.Error = "M3U8请求失败: "+err.Error(); return r }
	if resp.StatusCode != http.StatusOK {
		r.Error = fmt.Sprintf("M3U8 HTTP %d",resp.StatusCode); resp.Body.Close(); return r
	}
	body := make([]byte, 256*1024)
	n, readErr := resp.Body.Read(body)
	resp.Body.Close()
	if readErr != nil && n == 0 { r.Error = "读取播放列表失败: "+readErr.Error(); return r }
	playlist := string(body[:n])
	if !strings.Contains(playlist,"#EXTM3U") { r.Error = "响应不是有效 M3U8"; return r }
	r.Playlist = true
	scanner := bufio.NewScanner(strings.NewReader(playlist))
	var segment string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line,"#") { segment = line; break }
	}
	if segment == "" { r.Error = "播放列表未找到媒体分片"; return r }
	base, _ := url.Parse(playURL)
	ref, err := url.Parse(segment)
	if err != nil { r.Error = "分片地址解析失败"; return r }
	segmentURL := base.ResolveReference(ref).String()
	sreq, _ := http.NewRequest(http.MethodGet, segmentURL, nil)
	sreq.Header.Set("Range","bytes=0-1023")
	sreq.Header.Set("User-Agent","Mozilla/5.0")
	sresp, err := client.Do(sreq)
	if err != nil { r.Error = "TS抽样请求失败: "+err.Error(); return r }
	defer sresp.Body.Close()
	if sresp.StatusCode == http.StatusPartialContent || sresp.StatusCode == http.StatusOK {
		r.Segment = true
	} else {
		r.Error = fmt.Sprintf("TS分片抽样 HTTP %d",sresp.StatusCode)
	}
	r.Passed = r.Resolved && r.Playlist && r.Segment && freeListed
	if !freeListed && r.Error == "" { r.Error = "技术抽样通过，但尚未加入人工确认的免费频道名单" }
	return r
}
