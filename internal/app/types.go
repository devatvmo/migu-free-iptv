package app

type Category struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type Channel struct {
	Name     string `json:"name"`
	PID      string `json:"pid"`
	Category string `json:"category"`
	Group    string `json:"group,omitempty"`
}

type ProbeResult struct {
	Channel
	Resolved bool   `json:"resolved"`
	Playlist bool   `json:"playlist_ok"`
	Segment  bool   `json:"segment_ok"`
	FreeListed bool `json:"free_listed"`
	Passed   bool   `json:"passed"`
	PlayURL  string `json:"play_url,omitempty"`
	Error    string `json:"error,omitempty"`
	CheckedAt string `json:"checked_at"`
}

type Allowlist struct {
	Description string   `json:"description"`
	PIDs        []string `json:"pids"`
}
