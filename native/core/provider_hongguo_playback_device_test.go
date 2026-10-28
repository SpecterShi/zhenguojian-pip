package core

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type hongguoRepairTransport func(*http.Request) (*http.Response, error)

func (f hongguoRepairTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func hongguoRepairResponse(body string) *http.Response {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func TestHongguoPlaybackRegistrationSharedAndExact(t *testing.T) {
	var calls atomic.Int32
	d := &Downloader{client: &http.Client{Transport: hongguoRepairTransport(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.URL.String() != hongguoPlaybackRegisterURL || r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/octet-stream;tt-data=a" {
			t.Error("unexpected registration request")
		}
		return hongguoRepairResponse(`{"device_id_str":"9007199254740993","install_id":9007199254741993,"device_token":"do-not-retain"}`), nil
	})}}
	var wait sync.WaitGroup
	for range 20 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			id, err := d.hongguoPlaybackDevice(context.Background())
			if err != nil || id.deviceID != "9007199254740993" || id.installID != "9007199254741993" || id.cdid == "" {
				t.Errorf("identity mismatch: %v", err)
			}
		}()
	}
	wait.Wait()
	if calls.Load() != 1 {
		t.Fatalf("registration calls: %d", calls.Load())
	}
}

func TestHongguoPlaybackRegistrationFailureCachedAndRedacted(t *testing.T) {
	for _, body := range []string{`{"device_id":"0","install_id":"123"}`, `{"device_id":"123","install_id":"0"}`, `{"code":1,"device_id":"123","install_id":"456"}`, `private_token_value`} {
		calls := 0
		d := &Downloader{client: &http.Client{Transport: hongguoRepairTransport(func(*http.Request) (*http.Response, error) {
			calls++
			return hongguoRepairResponse(body), nil
		})}}
		for range 3 {
			_, err := d.hongguoPlaybackDevice(context.Background())
			if err == nil || err.Error() != "红果匿名播放设备注册失败，请重启应用后重试" {
				t.Fatalf("unsafe or missing registration error: %v", err)
			}
		}
		if calls != 1 {
			t.Fatalf("failed registration retried %d times", calls)
		}
	}
}

func TestHongguoPlaybackRegistrationCancellation(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	d := &Downloader{client: &http.Client{Transport: hongguoRepairTransport(func(r *http.Request) (*http.Response, error) {
		close(started)
		<-release
		if r.Context().Err() != nil {
			t.Error("one caller cancelled shared registration")
		}
		return hongguoRepairResponse(`{"device_id":"123","install_id":"456"}`), nil
	})}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := d.hongguoPlaybackDevice(ctx); done <- err }()
	<-started
	cancel()
	if !errors.Is(<-done, context.Canceled) {
		t.Fatal("cancelled caller did not return")
	}
	close(release)
	if _, err := d.hongguoPlaybackDevice(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestHongguoRegisteredInlineModel(t *testing.T) {
	for _, mode := range []string{"inline", "wrong-episode", "missing-key", "fallback"} {
		t.Run(mode, func(t *testing.T) {
			var registrations, models, fallback int
			d := &Downloader{client: &http.Client{Transport: hongguoRepairTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host == "log5-applog-lf.fqnovel.com" {
					registrations++
					return hongguoRepairResponse(`{"device_id":"123","install_id":"456"}`), nil
				}
				if r.URL.Path == "/fixture" {
					fallback++
					return hongguoRepairResponse(`{"code":0,"video_info":{"data":{"video_list":[{"main_url":"https://sample.fqnovelvod.com/legacy.mp4","codec_type":"h264","vwidth":1280,"vheight":720}]}}}`), nil
				}
				models++
				if r.URL.Query().Get("device_id") != "123" || r.URL.Query().Get("iid") != "456" || r.URL.Query().Get("cdid") == "" || r.Header.Get("X-Medusa") != "" {
					t.Error("incorrect registered playback request")
				}
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				vid := payload["mixed_video_id_map"].(map[string]any)["1004"].([]any)[0].(string)
				model := map[string]any{"video_list": []any{map[string]any{"main_url": "https://sample.fqnovelvod.com/current.mp4?format=hls", "video_meta": map[string]any{"codec_type": "bytevc1", "vwidth": 1080, "vheight": 1920}, "encrypt_info": map[string]any{"encrypt": false}}}}
				if mode == "missing-key" {
					model["video_list"].([]any)[0].(map[string]any)["encrypt_info"] = map[string]any{"encrypt": true}
				}
				if mode == "wrong-episode" {
					vid = "999"
				}
				if mode == "fallback" {
					model = map[string]any{"fallback_api": "https://api.fqnovel.com/fixture"}
				}
				body, _ := json.Marshal(map[string]any{"code": 0, "data": map[string]any{vid: map[string]any{"video_model": model}}})
				return hongguoRepairResponse(string(body)), nil
			})}}
			media, err := d.resolveHongguoMultiMedia(context.Background(), "800001")
			if mode == "wrong-episode" || mode == "missing-key" {
				if err == nil {
					t.Fatal("accepted invalid episode or missing key")
				}
			} else {
				quality := 1080
				if mode == "fallback" {
					quality = 720
				}
				if err != nil || !media.Direct || media.Quality != quality || len(media.Variants) != 1 {
					t.Fatalf("invalid media result: quality=%d direct=%v error=%v", media.Quality, media.Direct, err)
				}
			}
			_, _ = d.resolveHongguoMultiMedia(context.Background(), "800002")
			if registrations != 1 || models != 2 {
				t.Fatalf("registration/model count %d/%d", registrations, models)
			}
			if mode != "fallback" && fallback != 0 {
				t.Fatal("inline model fetched legacy fallback")
			}
		})
	}
}
