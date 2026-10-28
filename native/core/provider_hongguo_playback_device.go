// SPDX-License-Identifier: GPL-3.0-only
// Adapted from jackotom/hongguo-drama cd71e3ea7405d94b2de5c1b7e697c9dc5716c872,
// src/native/playback-device.js; protocol attribution: tsukiyo-zero/hongguoTV
// 388931852ddc614db8b00775a3dc54c5ede6307f and charlie901208/HGTV feat/phone-login.
package core

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const hongguoPlaybackRegisterURL = "https://log5-applog-lf.fqnovel.com/service/2/device_register/?tt_data=a&cronet_version=8d40f833_2026-03-03&ttnet_version=4.2.243.31-douyin&use_store_region_cookie=1"
const hongguoRegisteredPlaybackUA = "com.phoenix.read/71532 (Linux; U; Android 9; SM-N9860; Build/PQ3A.190705.10241111;tt-ok/3.12.13.20)"

type hongguoPlaybackIdentity struct {
	deviceID  string
	installID string
	cdid      string
}

type hongguoPlaybackRegistration struct {
	once     sync.Once
	ready    chan struct{}
	identity hongguoPlaybackIdentity
	err      error
}

func (d *Downloader) hongguoPlaybackDevice(ctx context.Context) (hongguoPlaybackIdentity, error) {
	if err := ctx.Err(); err != nil {
		return hongguoPlaybackIdentity{}, err
	}
	state := &d.hongguoPlayback
	state.once.Do(func() {
		state.ready = make(chan struct{})
		go func() {
			defer close(state.ready)
			registrationCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 8*time.Second)
			defer cancel()
			state.identity, state.err = d.registerHongguoPlaybackDevice(registrationCtx)
			if state.err != nil {
				state.err = errors.New("红果匿名播放设备注册失败，请重启应用后重试")
			}
		}()
	})
	select {
	case <-ctx.Done():
		return hongguoPlaybackIdentity{}, ctx.Err()
	case <-state.ready:
		return state.identity, state.err
	}
}

func hongguoPlaybackRegistrationBody(now int64, cdid string) ([]byte, error) {
	header := map[string]any{
		"display_name": "红果免费短剧", "update_version_code": 73932, "manifest_version_code": 73932,
		"app_version_minor": "7.3.9.32", "aid": 8662, "channel": "update_64", "package": "com.phoenix.read",
		"app_version": "7.3.9.32", "version_code": 73932, "sdk_version": "3.7.0-rc.34-fanqie-xiaoshuo",
		"sdk_target_version": 29, "git_hash": "3a88f4e", "os": "Android", "os_version": "16", "os_api": 36,
		"device_model": "25053RT47C", "device_brand": "Redmi", "device_manufacturer": "Redmi",
		"cpu_abi": "arm64-v8a", "release_build": "b5e187f", "density_dpi": 520, "display_density": "mdpi",
		"resolution": "1280x2772", "language": "zh", "timezone": 8, "access": "wifi", "not_request_sender": 0,
		"carrier": "CHINA MOBILE", "mcc_mnc": "46000", "rom": "eng.root.20231116.160340",
		"rom_version": "W528JS release-keys", "cdid": cdid,
		"sig_hash": "56a962410c494bbaf0b58dba20cae56f", "openudid": randomHex(8), "clientudid": uuidLike(),
		"ipv6_list": []string{}, "region": "CN", "tz_name": "Asia/Shanghai", "tz_offset": 28800,
		"sim_region": "cn", "sim_serial_number": []string{}, "oaid_may_support": false,
		"req_id": uuidLike(), "device_platform": "android",
		"custom":                map[string]any{"account_region_src": "fallback", "account_region": "cn", "host_bit": 64, "dragon_device_type": "phone"},
		"pre_installed_channel": "", "apk_first_install_time": now, "is_system_app": 0, "sdk_flavor": "china", "guest_mode": 0,
	}
	raw, err := json.Marshal(map[string]any{"magic_tag": "ss_app_log", "header": header, "_gen_time": now})
	if err != nil {
		return nil, err
	}
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(raw); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return nil, err
	}
	first := sha512.Sum512(random)
	salt := mustHex("4dd4c2e6b83162090e52b3c7a6733ba41cb2462b829ab58a196b39db57177524f49baf7f08e8d68d26a72e37c1a95a2f1f05a51892aef2949732b62a38aadd58")
	derived := sha512.Sum512(append(first[:], salt...))
	digest := sha512.Sum512(compressed.Bytes())
	plaintext := append(digest[:], compressed.Bytes()...)
	padding := aes.BlockSize - len(plaintext)%aes.BlockSize
	plaintext = append(plaintext, bytes.Repeat([]byte{byte(padding)}, padding)...)
	block, err := aes.NewCipher(derived[:16])
	if err != nil {
		return nil, err
	}
	ciphertext := make([]byte, len(plaintext))
	cipher.NewCBCEncrypter(block, derived[16:32]).CryptBlocks(ciphertext, plaintext)
	return append(append(mustHex("746305100000"), random...), ciphertext...), nil
}

func (d *Downloader) registerHongguoPlaybackDevice(ctx context.Context) (hongguoPlaybackIdentity, error) {
	identity := hongguoPlaybackIdentity{cdid: uuidLike()}
	now := time.Now().UnixMilli()
	body, err := hongguoPlaybackRegistrationBody(now, identity.cdid)
	if err != nil {
		return identity, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, hongguoPlaybackRegisterURL, bytes.NewReader(body))
	if err != nil {
		return identity, err
	}
	headers := map[string]string{
		"User-Agent": "com.phoenix.read/72232 (Linux; U; Android 16; zh_CN; 25053RT47C; Build/BP2A.250605.031.A3)",
		"Accept":     "application/json", "Content-Type": "application/octet-stream;tt-data=a",
		"log-encode-type": "gzip", "x-ss-dp": "8662", "x-ss-req-ticket": strconv.FormatInt(now, 10),
		"x-vc-bdturing-sdk-version": "4.0.3.cn", "lc": "101", "sdk-version": "2",
		"passport-sdk-version": "5051452", "x-neptune": "-8|50:51:59:30:40:47:49:39",
	}
	digest := md5.Sum(body)
	headers["x-ss-stub"] = strings.ToUpper(hex.EncodeToString(digest[:]))
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	client := *d.client
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(request)
	if err != nil {
		return identity, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return identity, errors.New("registration unavailable")
	}
	var result map[string]any
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64<<10))
	decoder.UseNumber()
	if err := decoder.Decode(&result); err != nil {
		return identity, err
	}
	if code := firstNonEmpty(mapString(result, "code", "Code"), "0"); code != "0" {
		return identity, errors.New("registration rejected")
	}
	identity.deviceID = mapString(result, "device_id_str", "device_id")
	identity.installID = mapString(result, "install_id_str", "install_id")
	for _, id := range []string{identity.deviceID, identity.installID} {
		if !hongguoNumericID.MatchString(id) || strings.Trim(id, "0") == "" {
			return identity, errors.New("invalid registration identity")
		}
	}
	return identity, nil
}

func (d *Downloader) fetchHongguoRegisteredModel(ctx context.Context, payload []byte) (map[string]any, error) {
	identity, err := d.hongguoPlaybackDevice(ctx)
	if err != nil {
		return nil, err
	}
	pairs := append([][2]string{}, hongguoMultiDevicePairsTemplate...)
	pairs = append(pairs, [2]string{"device_id", identity.deviceID}, [2]string{"iid", identity.installID},
		[2]string{"cdid", identity.cdid}, [2]string{"_rticket", strconv.FormatInt(time.Now().UnixMilli(), 10)})
	headers := map[string]string{"User-Agent": hongguoRegisteredPlaybackUA,
		"Accept": "application/json; charset=utf-8,application/x-protobuf", "Content-Type": "application/json; charset=UTF-8"}
	return hongguoMultiFetchJSON(ctx, d.client, hongguoMultiVideoAPI+"?"+hongguoPythonUrlEncode(pairs), http.MethodPost, headers, payload)
}
