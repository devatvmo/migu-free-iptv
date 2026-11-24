package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const catalogURL = "https://program-sc.miguvideo.com/live/v2/tv-data/"

func FetchCatalog(categories []Category) ([]Channel, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	seen := map[string]bool{}
	var channels []Channel
	for _, category := range categories {
		req, err := http.NewRequest(http.MethodGet, catalogURL+category.ID, nil)
		if err != nil { return nil, err }
		req.Header.Set("User-Agent", "Mozilla/5.0")
		req.Header.Set("Referer", "https://www.miguvideo.com/")
		resp, err := client.Do(req)
		if err != nil { return nil, fmt.Errorf("目录 %s 请求失败: %w", category.Name, err) }
		var root map[string]any
		decErr := json.NewDecoder(resp.Body).Decode(&root)
		resp.Body.Close()
		if decErr != nil { return nil, fmt.Errorf("目录 %s JSON解析失败: %w", category.Name, decErr) }
		body, _ := root["body"].(map[string]any)
		if body == nil { continue }
		list, _ := body["dataList"].([]any)
		for _, raw := range list {
			item, _ := raw.(map[string]any)
			if item == nil { continue }
			pid := firstString(item, "pID", "pid", "contId", "contentId")
			name := firstString(item, "name", "title", "contName")
			if pid == "" || name == "" || seen[pid] { continue }
			seen[pid] = true
			channels = append(channels, Channel{Name:name, PID:pid, Category:category.Name, Group:firstString(item,"group")})
		}
		time.Sleep(350 * time.Millisecond)
	}
	return channels, nil
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" { return strings.TrimSpace(s) }
			if n, ok := v.(float64); ok { return fmt.Sprintf("%.0f", n) }
		}
	}
	return ""
}
