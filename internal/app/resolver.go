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
	"strings"
	"time"
)

const appVersion = "2600034600"
const ddCalcuKeys = "cdabyzwxkl"

func md5hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// visitor720pDDCalcu reproduces the legacy Android guest-720p puData
// transformation used by the public hhzol/migu and vipxb/migu implementations.
func visitor720pDDCalcu(puData, pid, date string) string {
	if puData == "" || len(pid) < 7 || len(date) < 3 {
		return ""
	}
	pidDigit := pid[6] - '0'
	dateDigit := date[2] - '0'
	if pidDigit > 9 || dateDigit > 9 {
		return ""
	}

	out := make([]byte, 0, len(puData)+5)
	for i := 0; i < len(puData)/2; i++ {
		out = append(out, puData[len(puData)-i-1], puData[i])
		switch i {
		case 1:
			out = append(out, 'v')
		case 2:
			out = append(out, ddCalcuKeys[dateDigit])
		case 3:
			out = append(out, ddCalcuKeys[pidDigit])
		case 4:
			out = append(out, 'a')
		}
	}
	return string(out)
}

// sealVisitorURL adds the legacy guest-HD URL parameters when puData is present.
// Keep the dispatch URL intact: a static playlist should let the viewer's player
// perform network-dependent CDN dispatch rather than pinning a CNB-selected edge.
func sealVisitorURL(rawURL, pid string, now time.Time) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("解析咪咕播放地址失败: %w", err)
	}
	q := u.Query()
	puData := q.Get("puData")
	if puData == "" {
		return rawURL, nil
	}

	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	date := now.In(loc).Format("20060102150405")
	dd := visitor720pDDCalcu(puData, pid, date)
	if dd != "" {
		q.Set("ddCalcu", dd)
	}
	q.Set("sv", "10004")
	q.Set("ct", "android")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// ResolveVisitor requests a visitor-mode stream URL at the free 720p rate tier.
// This uses the legacy app version/signature required by community implementations
// for actual guest 720p; the returned URL is sealed with ddCalcu when puData exists.
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
	if err != nil {
		return "", err
	}
	req.Header.Set("AppVersion", appVersion)
	req.Header.Set("TerminalId", "android")
	req.Header.Set("X-UP-CLIENT-CHANNEL-ID", "2600034600-99000-201600010010028")
	req.Header.Set("ClientId", md5hex(strconv.FormatInt(time.Now().UnixMilli(), 10)))
	req.Header.Set("appCode", "miguvideo_default_android")
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("播放接口 HTTP %d: %s", resp.StatusCode, string(data))
	}
	var payload struct {
		RID  string `json:"rid"`
		Body struct {
			URLInfo struct {
				URL string `json:"url"`
			} `json:"urlInfo"`
		} `json:"body"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", err
	}
	if payload.Body.URLInfo.URL == "" {
		return "", fmt.Errorf("接口未返回播放地址 rid=%s", payload.RID)
	}
	sealed, err := sealVisitorURL(payload.Body.URLInfo.URL, pid, time.Now())
	if err != nil {
		return "", err
	}
	return sealed, nil
}
