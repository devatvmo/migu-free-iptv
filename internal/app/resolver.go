package app

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const appVersion = "2600034600"

func md5hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// ResolveVisitor requests a visitor-mode stream URL at the free 720p rate tier.
// This follows the public request shape used by community implementations;
// upstream signature parameters may change and must be revalidated periodically.
func ResolveVisitor(pid string) (string, error) {
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	base := ts + pid + appVersion[:8]
	salt := fmt.Sprintf("%06d25", rand.Intn(1000000))
	sign := md5hex(md5hex(base) + "2cac4f2c6c3346a5b34e085725ef7e33migu" + salt[:4])
	q := url.Values{}
	q.Set("sign", sign)
	q.Set("rateType", "3")
	q.Set("contId", pid)
	q.Set("timestamp", ts)
	q.Set("salt", salt)
	q.Set("super4k", "true")
	q.Set("h265N", "true")
	req, err := http.NewRequest(http.MethodGet, "https://play.miguvideo.com/playurl/v1/play/playurl?"+q.Encode(), nil)
	if err != nil { return "", err }
	req.Header.Set("AppVersion", appVersion)
	req.Header.Set("TerminalId", "android")
	req.Header.Set("X-UP-CLIENT-CHANNEL-ID", "2600034600-99000-201600010010028")
	req.Header.Set("ClientId", md5hex(strconv.FormatInt(time.Now().UnixMilli(),10)))
	req.Header.Set("appCode", "miguvideo_default_android")
	client := &http.Client{Timeout: 12*time.Second}
	resp, err := client.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil { return "", err }
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { return "", fmt.Errorf("播放接口 HTTP %d: %s", resp.StatusCode, string(data)) }
	var payload struct {
		RID string `json:"rid"`
		Body struct { URLInfo struct { URL string `json:"url"` } `json:"urlInfo"` } `json:"body"`
	}
	if err := json.Unmarshal(data, &payload); err != nil { return "", err }
	if payload.Body.URLInfo.URL == "" { return "", fmt.Errorf("接口未返回播放地址 rid=%s", payload.RID) }
	return payload.Body.URLInfo.URL, nil
}
