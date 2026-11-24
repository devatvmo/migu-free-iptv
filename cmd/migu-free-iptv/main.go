package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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

var (
	cctvArabic = regexp.MustCompile(`(?i)cctv[- _]*([1-9]|1[0-3])(?:[^0-9]|$)`)
	centralArabic = regexp.MustCompile(`中央([1-9]|1[0-3])(?:台|套|频道)?`)
	centralChinese = map[string]int{
		"中央一": 1, "中央二": 2, "中央三": 3, "中央四": 4, "中央五": 5,
		"中央六": 6, "中央七": 7, "中央八": 8, "中央九": 9, "中央十": 10,
		"中央十一": 11, "中央十二": 12, "中央十三": 13,
	}
)

func cctvNumber(name string) int {
	name = strings.TrimSpace(name)
	if m := cctvArabic.FindStringSubmatch(name); len(m) == 2 {
		var n int
		fmt.Sscanf(m[1], "%d", &n)
		return n
	}
	if m := centralArabic.FindStringSubmatch(name); len(m) == 2 {
		var n int
		fmt.Sscanf(m[1], "%d", &n)
		return n
	}
	for label, n := range centralChinese {
		if strings.Contains(name, label) {
			return n
		}
	}
	return 0
}

func main() {
	catPath := flag.String("categories", "config/categories.json", "频道分类配置")
	out := flag.String("out", "dist", "输出目录")
	delay := flag.Duration("delay", 1200*time.Millisecond, "频道请求间隔")
	flag.Parse()

	var cats []app.Category
	if err := readJSON(*catPath, &cats); err != nil {
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

	// Keep only CCTV-1 through CCTV-13. Do not apply the free/paid allowlist
	// at this stage: the user will test these candidate streams on the TV box.
	selected := make(map[int]app.Channel, 13)
	for _, ch := range channels {
		n := cctvNumber(ch.Name)
		if n >= 1 && n <= 13 {
			if _, exists := selected[n]; !exists {
				selected[n] = ch
			}
		}
	}

	results := make([]app.ProbeResult, 0, 13)
	urls := make(map[int]string, 13)
	for n := 1; n <= 13; n++ {
		ch, ok := selected[n]
		if !ok {
			fmt.Printf("CCTV-%d: 目录中未找到\n", n)
			continue
		}
		result := app.Probe(ch, false)
		results = append(results, result)
		if result.Resolved {
			urls[n] = result.PlayURL
		}
		fmt.Printf("CCTV-%d %s (%s): dispatch=%t playback-verified=false",
			n, ch.Name, ch.PID, result.Resolved)
		if result.Error != "" {
			fmt.Printf(" error=%s", result.Error)
		}
		fmt.Println()
		time.Sleep(*delay)
	}

	var m3u strings.Builder
	m3u.WriteString("#EXTM3U\n")
	var txt strings.Builder
	for n := 1; n <= 13; n++ {
		ch, ok := selected[n]
		playURL, resolved := urls[n]
		if !ok || !resolved {
			continue
		}
		fmt.Fprintf(&m3u, "#EXTINF:-1 tvg-name="%s" group-title="央视",%s\n%s\n", ch.Name, ch.Name, playURL)
		fmt.Fprintf(&txt, "%s,%s\n", ch.Name, playURL)
	}

	if err := os.WriteFile(filepath.Join(*out, "cctv-1-13.m3u"), []byte(m3u.String()), 0644); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(filepath.Join(*out, "cctv-1-13.txt"), []byte(txt.String()), 0644); err != nil {
		fatal(err)
	}
	if err := writeJSON(filepath.Join(*out, "cctv-probe-report.json"), results); err != nil {
		fatal(err)
	}

	resolvedCount := len(urls)
	summary := map[string]any{
		"catalog_candidate_count": len(channels),
		"cctv_target_count": 13,
		"cctv_found_count": len(selected),
		"cctv_dispatch_url_count": resolvedCount,
		"playback_verified_count": 0,
		"checked_at": time.Now().UTC().Format(time.RFC3339),
		"rate_type": 3,
		"visitor_mode": true,
		"note": "本轮仅生成中央1至13台候选调度地址；未判断免费/会员属性，也未在 CNB 端请求 M3U8 或媒体分片。必须在目标电视盒子网络中逐台实测。",
	}
	if err := writeJSON(filepath.Join(*out, "summary.json"), summary); err != nil {
		fatal(err)
	}
	sort.Ints([]int{})
	fmt.Printf("完成：找到 %d/13 个频道，成功获取 %d 个调度地址。\n", len(selected), resolvedCount)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
