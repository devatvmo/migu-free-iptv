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
	b, err := os.ReadFile(path); if err != nil { return err }
	return json.Unmarshal(b,target)
}
func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v,"","  "); if err != nil { return err }
	return os.WriteFile(path,append(b,'\n'),0644)
}

func main() {
	catPath := flag.String("categories","config/categories.json","频道分类配置")
	allowPath := flag.String("allowlist","config/free_allowlist.json","人工确认免费频道名单")
	out := flag.String("out","dist","输出目录")
	delay := flag.Duration("delay",1200*time.Millisecond,"频道探测间隔")
	flag.Parse()
	var cats []app.Category
	var allow app.Allowlist
	if err:=readJSON(*catPath,&cats); err!=nil { fatal(err) }
	if err:=readJSON(*allowPath,&allow); err!=nil { fatal(err) }
	if err:=os.MkdirAll(*out,0755); err!=nil { fatal(err) }
	channels,err:=app.FetchCatalog(cats); if err!=nil { fatal(err) }
	if err:=writeJSON(filepath.Join(*out,"candidates.json"),channels); err!=nil { fatal(err) }
	allowed:=map[string]bool{}; for _,pid:=range allow.PIDs { allowed[pid]=true }
	results:=make([]app.ProbeResult,0,len(channels))
	var m3u strings.Builder
	m3u.WriteString("#EXTM3U\n")
	var txt strings.Builder
	for i,ch:=range channels {
		result:=app.Probe(ch,allowed[ch.PID])
		results=append(results,result)
		if result.Passed {
			fmt.Fprintf(&m3u,"#EXTINF:-1 group-title=\"%s\",%s\n%s\n",ch.Category,ch.Name,result.PlayURL)
			fmt.Fprintf(&txt,"%s,%s\n",ch.Name,result.PlayURL)
		}
		fmt.Printf("[%d/%d] %s (%s): resolved=%t playlist=%t segment=%t free-listed=%t\n",i+1,len(channels),ch.Name,ch.PID,result.Resolved,result.Playlist,result.Segment,result.FreeListed)
		time.Sleep(*delay)
	}
	if err:=writeJSON(filepath.Join(*out,"probe-report.json"),results); err!=nil { fatal(err) }
	if err:=os.WriteFile(filepath.Join(*out,"free.m3u"),[]byte(m3u.String()),0644); err!=nil { fatal(err) }
	if err:=os.WriteFile(filepath.Join(*out,"free.txt"),[]byte(txt.String()),0644); err!=nil { fatal(err) }
	passed:=0; for _,r:=range results { if r.Passed { passed++ } }
	summary:=map[string]any{"candidate_count":len(channels),"published_count":passed,"checked_at":time.Now().UTC().Format(time.RFC3339),"rate_type":3,"visitor_mode":true,"note":"HTTP抽样成功不等于完整连续播放；免费资格仅由人工allowlist确认"}
	if err:=writeJSON(filepath.Join(*out,"summary.json"),summary); err!=nil { fatal(err) }
}
func fatal(err error) { fmt.Fprintln(os.Stderr,err); os.Exit(1) }
