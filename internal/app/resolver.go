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

// visitor720pDDCalcu reproduces the legacy Android guest-720p puData transformation.
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

// resolveDispatchURL mirrors the bounded manual-redirect strategy used by
// public Migu clients. The upstream bofang dispatch response is retried;
// ordinary redirects are resolved and returned. Unlike some upstream loops,
// every attempt is bounded, including responses without a Location header.
func resolveDispatchURL(rawURL string, client *http.Client, attempts int, pause time.Duration) (string, error) {
	if attempts < 1 {
		attempts = 1
	}
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("无效播放地址: %q", rawURL)
	}
	noRedirect := *client
	noRedirect.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		req, reqErr := http.NewRequest(http.MethodGet, u.String(), nil)
		if reqErr != nil {
			return "", reqErr
		}
		req.Header.Set("User-Agent", "Mozilla/5.0")
		resp, doErr := noRedirect.Do(req)
		if doErr != nil {
			lastErr = doErr
		} else {
			location := strings.TrimSpace(resp.Header.Get("Location"))
			status := resp.StatusCode
			resp.Body.Close()
			if status >= 200 && status < 300 {
				return u.String(), nil
			}
			if status >= 300 && status < 400 && location != "" {
				if strings.HasPrefix(strings.ToLower(location), "http://bofang") {
					lastErr = fmt.Errorf("咪咕 bofang 调度尚未返回最终地址")
				} else {
					next, parseErr := u.Parse(location)
					if parseErr != nil {
						return "", fmt.Errorf("解析重定向地址失败: %w", parseErr)
					}
					if next.Scheme != "http" && next.Scheme != "https" {
						return "", fmt.Errorf("重定向协议不受支持: %s", next.Scheme)
					}
					return next.String(), nil
				}
			} else {
				lastErr = fmt.Errorf("播放地址探测 HTTP %d，未获得有效重定向", status)
			}
		}
		if i+1 < attempts && pause > 0 {
			time.Sleep(pause)
		}
	}
	if lastErr != nil {
		return "", fmt.Errorf("播放地址重定向探测失败（尝试 %d 次）: %w", attempts, lastErr)
	}
	return "", fmt.Errorf("播放地址重定向探测失败")
}

// ResolveVisitor requests a visitor-mode stream URL at the free 720p rate tier.
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
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	resp.Body.Close()
	if readErr != nil {
		return "", readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("播放接口 HTTP %d: %s", resp.StatusCode, string(data))
	}
	var payload struct {
		RID string `json:"rid"`
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

	// First resolve Migu's dispatch redirect, then apply puData/ddCalcu to
	// the actual returned CDN URL. This ordering matches public guest clients.
	finalURL, err := resolveDispatchURL(payload.Body.URLInfo.URL, client, 6, 150*time.Millisecond)
	if err != nil {
		return "", err
	}
	return sealVisitorURL(finalURL, pid, time.Now())
}
