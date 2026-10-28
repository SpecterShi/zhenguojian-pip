package core

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"
)

func TestProbeProxyMedia(t *testing.T) {
	if os.Getenv("HG_PROBE") == "" {
		t.Skip("probe disabled")
	}
	engine, err := newNativeEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d := engine.downloader
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	media, err := d.resolveHongguoMultiMedia(ctx, "7503780574541122622")
	if err != nil {
		t.Fatalf("取流失败: %v", err)
	}
	// 直连
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, media.URL, nil)
	mediaRequestHeaders(req, media.Referer)
	req.Header.Set("Range", "bytes=0-1023")
	resp, err := d.doMediaRequest(req)
	if err != nil {
		fmt.Println("直连 NETERR", err)
	} else {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 16))
		resp.Body.Close()
		fmt.Printf("直连 -> %d head=%x\n", resp.StatusCode, b)
	}
	// 经 Clash 代理
	proxyClient := &http.Client{Transport: &http.Transport{Proxy: func(*http.Request) (*url.URL, error) {
		return url.Parse("http://127.0.0.1:7897")
	}}, Timeout: 20 * time.Second}
	req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, media.URL, nil)
	mediaRequestHeaders(req2, media.Referer)
	req2.Header.Set("Range", "bytes=0-1023")
	resp2, err := proxyClient.Do(req2)
	if err != nil {
		fmt.Println("经代理 NETERR", err)
	} else {
		b2, _ := io.ReadAll(io.LimitReader(resp2.Body, 16))
		resp2.Body.Close()
		fmt.Printf("经代理 -> %d head=%x\n", resp2.StatusCode, b2)
	}
}
