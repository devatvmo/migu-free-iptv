package app

import "time"

// Probe only obtains the original visitor dispatch URL. The CNB runner must
// not fetch the M3U8 or media segments: those requests need to originate from
// the end user's mainland-China TV box network. Publishable means only that
// the URL was obtained and the PID is on the manually reviewed free allowlist;
// it does not mean playback has been verified.
func Probe(ch Channel, freeListed bool) ProbeResult {
	r := ProbeResult{
		Channel:   ch,
		FreeListed: freeListed,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
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
