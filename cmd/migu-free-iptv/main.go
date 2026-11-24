package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devatvmo/migu-free-iptv/internal/app"
)

func readJSON(path string, target any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, target)
}
func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0644)
}

func main() {
	catPath := flag.String("categories", "config/categories.json", "频道分类配置")
	allowPath := flag.String("allowlist", "config/free_allowlist.json", "人工确认免费频道名单")
	out := flag.String("out", "dist", "输出目录")
	delay := flag.Duration("delay", 1200*time.Millisecond, "频道请求间隔")
	flag.Parse()
	var cats []app.Category
	var allow app.Allowlist
	if err := readJSON(*catPath, &cats); err != nil {
		fatal(err)
	}
	if err := readJSON(*allowPath, &allow); err != nil {
		fatal(err)
	}
	if err := os.MkdirAll(*out, 0755); err != nil {
		fatal(err)
	}
	channels, err := app.FetchCatalog(cats)
	if err != nil {
		fatal(err)
	}
	if err := writeJSON(filepath.Join(*out, "candidates.json"), channels); err != nil {
		fatal(err)
	}
	allowed := map[string]bool{}
	for _, pid := range allow.PIDs {
		allowed[pid] = true
	}
	results := make([]app.ProbeResult, 0, len(channels))
	var m3u strings.Builder
	m3u.WriteString("#EXTM3U\n")
	var txt strings.Builder
	for i, ch := range channels {
		result := app.Probe(ch, allowed[ch.PID])
		results = append(results, result)
		if result.Publishable {
			fmt.Fprintf(&m3u, "#EXTINF:-1 group-title=\"%s\",%s\n%s\n", ch.Category, ch.Name, result.PlayURL)
			fmt.Fprintf(&txt, "%s,%s\n", ch.Name, result.PlayURL)
		}
		fmt.Printf("[%d/%d] %s (%s): dispatch=%t free-listed=%t publishable=%t playback-verified=false\n",
			i+1, len(channels), ch.Name, ch.PID, result.Resolved, result.FreeListed, result.Publishable)
		time.Sleep(*delay)
	}
	if err := writeJSON(filepath.Join(*out, "probe-report.json"), results); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(filepath.Join(*out, "free.m3u"), []byte(m3u.String()), 0644); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(filepath.Join(*out, "free.txt"), []byte(txt.String()), 0644); err != nil {
		fatal(err)
	}
	publishable := 0
	for _, r := range results {
		if r.Publishable {
			publishable++
		}
	}
	summary := map[string]any{
		"candidate_count": len(channels),
		"published_count": publishable,
		"playback_verified_count": 0,
		"checked_at": time.Now().UTC().Format(time.RFC3339),
		"rate_type": 3,
		"visitor_mode": true,
		"note": "本任务仅获取原始游客调度地址；未在 CNB 端请求 M3U8 或媒体分片。published 仅表示 PID 在人工免费名单中，不代表电视盒子已验证可播放。",
	}
	if err := writeJSON(filepath.Join(*out, "summary.json"), summary); err != nil {
		fatal(err)
	}
}
func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
