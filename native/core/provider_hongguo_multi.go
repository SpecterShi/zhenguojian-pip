package core

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	mrand "math/rand"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const hongguoMultiVideoAPI = "https://api5-normal-sinfonlineb.fqnovel.com/novel/player/multi_video_model/v1/"

const hongguoMultiUserAgent = "com.phoenix.read/71332 (Linux; U; Android 16; zh_CN; 25053RT47C; Build/BP2A.250605.031.A3; Cronet/TTNetVersion:04657795 2026-01-23 QuicVersion:c67e9834 2025-09-08)"

var hongguoMultiProcessNonce = func() string {
	raw := make([]byte, 18)
	_, _ = rand.Read(raw)
	return base64.RawURLEncoding.EncodeToString(raw)
}()

var hongguoMultiProcessStarted = time.Now().UnixMilli()

var (
	hongguoMultiRandomUint32 = func() uint32 { return mrand.Uint32() }
	hongguoMultiRandomInt    = func(n int) int { return mrand.Intn(n) }
	hongguoMultiNowUnix      = func() int64 { return time.Now().Unix() }
)

func hongguoMultiLocalID() string {
	raw := make([]byte, 6)
	_, _ = rand.Read(raw)
	value := new(big.Int).SetBytes(raw)
	value.Add(value, big.NewInt(1_000_000_000_000_000))
	return value.String()
}

func rol32(value uint32, count int) uint32 {
	n := uint(count & 31)
	return value<<n | value>>(32-n)
}

func ror32(value uint32, count int) uint32 {
	n := uint(count & 31)
	return value>>n | value<<(32-n)
}

func ror64(value uint64, count int) uint64 {
	n := uint(count & 63)
	return value>>n | value<<(64-n)
}

func hongguoLe32(value uint32) []byte {
	out := make([]byte, 4)
	binary.LittleEndian.PutUint32(out, value)
	return out
}

func hongguoBe32(value uint32) []byte {
	out := make([]byte, 4)
	binary.BigEndian.PutUint32(out, value)
	return out
}

func hongguoMultiSM3(input []byte) []byte {
	sum := hongguoSM3(input)
	return sum[:]
}

func hongguoGetIv(iv uint32, data []byte) uint32 {
	value := iv
	for i := 0; i < len(data); i++ {
		if i%2 == 0 {
			value = (value >> 4) ^ value ^ (value << 6) ^ uint32(data[i])
		} else {
			value = ^((value >> 7) ^ value ^ (uint32(data[i]) | (value << 12)))
		}
	}
	return value
}

func hongguoSumMd5(data []byte) uint32 {
	check := int32(0x20220420)
	for i := 0; i < 12; i++ {
		var temp int32
		if i%2 == 0 {
			temp = int32(uint32(check)>>3) ^ check
		} else {
			temp = int32(uint32(check)>>5) ^ check
		}
		if i%2 == 0 {
			check = int32(data[i]) ^ (check << 7)
		} else {
			check = int32(data[i]) | (check << 11)
		}
		if i%2 != 0 {
			check = ^check
		}
		check ^= temp
	}
	return uint32((check | 4) ^ 0x1000000)
}

var hongguoBranch2SV = [64]uint32{0xa7aefe20, 0x7149f1d6, 0x47e4ca07, 0xe9b58f67, 0x93b924de, 0xc614d0f5, 0x38afe0ef, 0xb2bbad73, 0xe24444c3, 0x9d3aec9b, 0xdf7b37e4, 0xd8b16d40, 0xf8ac31b8, 0x76b9a90b, 0x31d833ee, 0x953fce64, 0x353595a4, 0x4609c13b, 0x36925008, 0x8c6d0925, 0x5df5c177, 0x1cfbf52b, 0x8a4fa7f0, 0x114ca35e, 0x8193f984, 0x7a7a8733, 0x316ab4d5, 0x3c20cfc9, 0xa6d84453, 0x3a18500c, 0x798ec47a, 0x97a76b28, 0x66c4ff96, 0x51716443, 0x0dd2fc3b, 0xb5696da7, 0xbbeb3ac5, 0x5c53d204, 0xd32608ce, 0x7279b9ec, 0xf4188ecf, 0xf7d793db, 0x332cc491, 0xab76ae15, 0x9bebe727, 0x18a01384, 0x5be9f8a7, 0x5f90a754, 0x39b663c0, 0x36673c83, 0x7c92f514, 0x9d7d94d7, 0xe2e8d9aa, 0x5f7e9ea9, 0x7abd4551, 0x569e05da, 0x40a25632, 0x3df5a9a5, 0xbab37d80, 0x454286dc, 0x3f5d4e78, 0x3d7b75d, 0xb1fe4af7, 0xa5ab26a3}

var hongguoBranch2Orders, _ = hex.DecodeString("0f0704000908030a060b050d0e010c020f05080c0009020103070e060b0a0d04060500070c000a04080f010b0d09020e060b02050403080101070a000d0c090e0d070e0f0b0208030c0509010004060a0d0902060f0b0a040807000c0503010e0c090f07060f030e020d0405010b0a000c050a090e0802040407030f01060b000b0f0408020a0700090d06010e03050c0b060a0508020c03070f0e090d0001040906080f050800040a0b030d01020c0e090d0c0604070a03030f00080105020e01000d0f090a0b0e0402080703060c0501080a0c0f0905060b000304020e070d04080f000c0f0e0d0a01060207090503040205080d0b0a0606000e0f070c09030a08040f000b01060d0c0709030e05020a070b050f00020e0108030d0c0609040d070f08050f06040b0a0e0c090002030d0c020704010b0e0e08060f09050003")

func hongguoMd5V3Step(kind int, a, b, c, d, m uint32, shift int, constant uint32) uint32 {
	var f uint32
	switch kind {
	case 0:
		f = (b & c) | (^b & d)
	case 1:
		f = (b & d) | (c & ^d)
	case 2:
		f = b ^ c ^ d
	default:
		f = c ^ (b | ^d)
	}
	return b + rol32(a+f+m+constant, shift)
}

func hongguoMd5SumV3(message []byte, countV2 uint32, orders []byte, countV1 uint32) []byte {
	sv := make([]uint32, 64)
	for i, value := range hongguoBranch2SV {
		sv[i] = ror32(value, int(countV1))
	}
	start := [4]uint32{ror32(0x79e0f2fb, int(countV2)), ror32(0xc8b52570, int(countV2)), ror32(0xebc2f8cd, int(countV2)), ror32(0x7c104d93, int(countV2))}
	endCount := int((countV2 + 6) & 255)
	end := [4]uint32{ror32(0x19be4866, endCount), ror32(0xe85986b4, endCount), ror32(0xe19b326e, endCount), ror32(0x71d1d7d4, endCount)}
	m := make([]uint32, 16)
	for i := 0; i < 16; i++ {
		m[i] = binary.LittleEndian.Uint32(message[i*4:])
	}
	a, b, c, d := start[0], start[1], start[2], start[3]
	shiftSets := [4][16]int{
		{7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22},
		{5, 9, 14, 20, 5, 9, 14, 20, 5, 9, 14, 20, 5, 9, 14, 20},
		{4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23},
		{6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21},
	}
	for kind := 0; kind < 4; kind++ {
		offset := kind * 16
		for i := 0; i < 16; i++ {
			phase := i & 3
			index := orders[offset+i]
			shift := shiftSets[kind][i]
			var next uint32
			switch phase {
			case 0:
				next = hongguoMd5V3Step(kind, a, b, c, d, m[index], shift, sv[offset+i])
			case 1:
				next = hongguoMd5V3Step(kind, d, a, b, c, m[index], shift, sv[offset+i])
			case 2:
				next = hongguoMd5V3Step(kind, c, d, a, b, m[index], shift, sv[offset+i])
			default:
				next = hongguoMd5V3Step(kind, b, c, d, a, m[index], shift, sv[offset+i])
			}
			switch phase {
			case 0:
				a = next
			case 1:
				d = next
			case 2:
				c = next
			default:
				b = next
			}
		}
	}
	ret := make([]byte, 16)
	binary.LittleEndian.PutUint32(ret[0:], (start[0]+a)^end[0])
	binary.LittleEndian.PutUint32(ret[4:], (start[1]+b)^end[1])
	binary.LittleEndian.PutUint32(ret[8:], (start[2]+c)^end[2])
	binary.LittleEndian.PutUint32(ret[12:], (start[3]+d)^end[3])
	return append(ret, hongguoLe32(hongguoSumMd5(ret))...)
}

func hongguoBranch2F13(iv uint32, querySm3, bodyMd5, tsBytes []byte, khronos uint32) []byte {
	v := int(iv&13) * 86
	ivV0 := ((v >> 15) & 255) + ((v >> 8) & 255)
	n1 := [5]uint32{0x8980f29b, 0xeb549c7f, 0xb08726db, 0xd40cb5e6, 0xe8f559e4}[ivV0]
	countV1 := (n1 + khronos + 1) & 255
	countV2 := n1 + khronos
	shift := int((countV2 + 5) & 7)
	seed := [8]byte{0x84, 0x96, 0x77, 0x9d, 0xd4, 0x15, 0x0b, 0xf8}
	pad := make([]byte, 8)
	for i, value := range seed {
		wide := uint32(value) | uint32(value)<<8
		pad[i] = byte((wide >> uint(shift)) & 255)
	}
	input := append([]byte{}, querySm3...)
	input = append(input, bodyMd5...)
	input = append(input, tsBytes...)
	input = append(input, pad...)
	tail, _ := hex.DecodeString("a0010000")
	input = append(input, tail...)
	return hongguoMd5SumV3(input, countV2, hongguoBranch2Orders[ivV0<<6:(ivV0+1)<<6], countV1)
}

func hongguoHashF13(querySm3, bodyMd5, tsBytes []byte, khronos uint32) []byte {
	iv := hongguoGetIv(hongguoGetIv(hongguoGetIv(0x20230928, querySm3), bodyMd5), tsBytes)
	low := int(iv & 15)
	ivV0 := (low * 171) >> 9
	branch := low - ivV0*3
	if branch == 2 {
		return hongguoBranch2F13(iv, querySm3, bodyMd5, tsBytes, khronos)
	}
	if branch != 0 {
		combined := append(append([]byte{}, querySm3...), bodyMd5...)
		return append(append(append([]byte{}, querySm3[:16]...), bodyMd5...), hongguoLe32(hongguoSumMd5(combined))...)
	}
	ivV1List := [6]uint32{0xc4a78580, 0xb3c0fd39, 0xc58c5686, 0xc9aa3ba7, 0xf5a7adf2, 0x963c2ed1}
	ivV1 := ivV1List[ivV0]
	countV1 := (ivV1 + khronos + 1) & 255
	countV2 := ivV1 + khronos
	tt02 := [64]uint32{0xebb64faf, 0x7aadcc2, 0xcf3187bf, 0xe01138ff, 0x6d0bfcff, 0x5a30a3be, 0xb41ad638, 0x34180eb8, 0xf233eb6f, 0xb1a584cc, 0xccc30dc7, 0x47d1db51, 0xd55653de, 0x70a84fa1, 0x57473c12, 0xf76f0288, 0x2c077f0a, 0xda0dcad0, 0xfbb86f6c, 0xfdc4cf00, 0x688a020d, 0xe676c6a6, 0x8cd6338b, 0x1a3c8d0e, 0xcce8b06b, 0x6ad0ed0b, 0xa0522717, 0xdc71ac83, 0x2285db71, 0xd5b4dda6, 0x736f8650, 0x6560306c, 0x617ce2a6, 0xe423417e, 0xa40e143, 0x544e4032, 0x88dffb2a, 0x716c1ae0, 0x4c467a88, 0x5b23bb3, 0xe1d0b866, 0xbaa3dcb8, 0xae3374d3, 0xc3381a50, 0x1702f75b, 0xfe6da368, 0xf0b4cf48, 0x4e0ffbb8, 0x72aad10d, 0x26c53a3d, 0xf2bce0f6, 0xb4557581, 0x4a257fdd, 0x8c3182a2, 0xab0b3b86, 0x3d5dfb14, 0x4f103634, 0xd37b52d7, 0x444eff16, 0xeb0a33d1, 0x6ca86f6e, 0x284ba7, 0x8387cfa, 0x5fb37586}
	tt := make([]uint32, 64)
	for i, value := range tt02 {
		tt[i] = ror32(value, int(countV1))
	}
	n0 := int((countV2 + 2) & 7)
	seed := [4]byte{0xfa, 0x45, 0x61, 0xd7}
	pad := make([]byte, 4)
	for i, value := range seed {
		wide := uint32(value) | uint32(value)<<8
		pad[i] = byte((wide >> uint(n0)) & 255)
	}
	head, _ := hex.DecodeString("00000000000001a0")
	data := append(append(append(append(append([]byte{}, querySm3...), bodyMd5...), tsBytes...), pad...), head...)
	di := make([]uint32, 128)
	for i := 0; i < 16; i++ {
		di[i] = binary.BigEndian.Uint32(data[i*4:])
	}
	di0 := di[0]
	for i := 0; i < 112; i++ {
		di1 := di[i+1]
		di14 := di[i+14]
		r1 := rol32(di1, 14) ^ rol32(di1, 25) ^ (di1 >> 3)
		r2 := rol32(di14, 13) ^ rol32(di14, 15) ^ (di14 >> 10)
		di = append(di, di0+di[i+9]+r1+r2)
		di0 = di1
	}
	init := [8]uint32{ror32(0x7aba4fc8, int(countV2&31)), ror32(0x67166507, int(countV2&31)), ror32(0x6403fa00, int(countV2&31)), ror32(0x340f512f, int(countV2&31)), ror32(984304912, int(countV2&31)), ror32(3005047866, int(countV2&31)), ror32(2874125293, int(countV2&31)), ror32(2152413264, int(countV2&31))}
	chosenSets := [6][11]int{
		{101, 5, 7, 6, 3, 2, 1, 0, 5, 4, 3},
		{96, 0, 6, 7, 5, 3, 2, 1, 5, 4, 4},
		{96, 7, 6, 2, 1, 4, 0, 5, 4, 3, 5},
		{99, 3, 6, 2, 4, 5, 1, 0, 0, 7, 6},
		{96, 0, 5, 6, 7, 3, 1, 2, 5, 4, 4},
		{100, 2, 0, 3, 5, 4, 6, 7, 2, 1, 5},
	}
	chosen := chosenSets[ivV0]
	d := append([]uint32{}, init[:]...)
	for i := 0; i < chosen[0]; i++ {
		base := int(ivV1) + i
		n0v := di[base&127]
		n1 := ((d[chosen[3]] ^ d[chosen[4]]) & d[chosen[1]]) ^ d[chosen[3]]
		n2 := rol32(d[chosen[1]], 26) ^ rol32(d[chosen[1]], 21) ^ rol32(d[chosen[1]], 7)
		n4 := n0v + n1 + n2 + tt[base&63] + d[chosen[5]]
		n5 := rol32(d[chosen[2]], 30) ^ rol32(d[chosen[2]], 19) ^ rol32(d[chosen[2]], 10)
		n6 := (d[chosen[2]] & d[chosen[6]]) | ((d[chosen[2]] | d[chosen[6]]) & d[chosen[7]])
		n7 := n5 + n6
		old := d[chosen[9]]
		last := d[len(d)-1]
		copy(d[1:], d[:len(d)-1])
		d[0] = last
		d[chosen[10]] = n7 + n4
		d[chosen[8]] = old + n4
	}
	ret := make([]byte, 32)
	for i := 0; i < 8; i++ {
		binary.BigEndian.PutUint32(ret[i*4:], d[i]+init[i])
	}
	for i := 0; i < 16; i++ {
		ret[i] ^= ret[i+16]
	}
	return append(ret[:16:16], hongguoLe32(hongguoSumMd5(ret[:16]))...)
}

func hongguoReverseBits(value byte) byte {
	var result byte
	for i := 0; i < 8; i++ {
		result = result<<1 | (value>>i)&1
	}
	return result
}

func hongguoRC4Gorgon(data, key []byte) []byte {
	s := make([]int, 256)
	for i := range s {
		s[i] = i
	}
	j := 0
	for i := 0; i < 256; i++ {
		j = (j + s[i] + int(key[i%len(key)])) & 255
		s[i] = s[j]
	}
	i, jj := 0, 0
	out := make([]byte, len(data))
	for index, value := range data {
		i++
		x := s[i]
		jj += x
		y := s[jj&255]
		s[i] = y
		out[index] = value ^ byte(s[(y+y)&255])
	}
	return out
}

func hongguoXGorgon(query string, body []byte, timestamp uint32, random int) string {
	sdk := make([]byte, 4)
	binary.LittleEndian.PutUint32(sdk, 67503104)
	queryHash := md5.Sum([]byte(query))
	bodyHash := make([]byte, 4)
	if body != nil {
		sum := md5.Sum(body)
		copy(bodyHash, sum[:4])
	}
	input := append([]byte{}, queryHash[:4]...)
	input = append(input, bodyHash...)
	input = append(input, make([]byte, 4)...)
	input = append(input, sdk...)
	input = append(input, hongguoBe32(timestamp)...)
	key := []byte{0x4a, 0x40, 0x16, byte(random>>8) & 255, 0x47, 0x6c, 0x01, byte(random & 255)}
	out := hongguoRC4Gorgon(input, key)
	for i := range out {
		value := (out[i] >> 4) | (out[i] << 4)
		next := byte(0)
		if i+1 < len(out) {
			next = out[i+1]
		} else {
			next = out[0]
		}
		out[i] = ^(hongguoReverseBits((next^value)&255) ^ 20) & 255
	}
	prefix, _ := hex.DecodeString("8404")
	full := append(prefix, byte(random&255), byte(random>>8)&255, 0x40, 0x01)
	full = append(full, out...)
	return hex.EncodeToString(full)
}

type hongguoProtoField struct {
	tag   uint64
	wire  byte
	bytes []byte
	num   uint64
	float float32
}

func hongguoVarint(value uint64) []byte {
	var out []byte
	for value > 127 {
		out = append(out, byte(value&127|128))
		value >>= 7
	}
	return append(out, byte(value))
}

func hongguoProtoFieldBytes(tag uint64, value []byte) []byte {
	if value == nil {
		return nil
	}
	key := hongguoVarint(tag<<3 | 2)
	length := hongguoVarint(uint64(len(value)))
	return append(append(key, length...), value...)
}

func hongguoProtoFieldString(tag uint64, value string) []byte {
	if value == "" {
		return nil
	}
	return hongguoProtoFieldBytes(tag, []byte(value))
}

func hongguoProtoFieldSint(tag uint64, value int64) []byte {
	var zigzag uint64
	if value < 0 {
		zigzag = uint64(-value)*2 - 1
	} else {
		zigzag = uint64(value) * 2
	}
	return append(hongguoVarint(tag<<3), hongguoVarint(zigzag)...)
}

func hongguoProtoFieldInt(tag uint64, value uint64) []byte {
	return append(hongguoVarint(tag<<3), hongguoVarint(value)...)
}

func hongguoProtoFieldFloat(tag uint64, value float32) []byte {
	out := hongguoVarint(tag<<3 | 5)
	raw := math.Float32bits(value)
	body := make([]byte, 4)
	binary.LittleEndian.PutUint32(body, raw)
	return append(out, body...)
}

func hongguoProto(fields ...[]byte) []byte {
	var out []byte
	for _, field := range fields {
		out = append(out, field...)
	}
	return out
}

func hongguoDeviceProto(deviceID string) []byte {
	return hongguoProto(
		hongguoProtoFieldSint(1, 1),
		hongguoProtoFieldSint(2, 2),
		hongguoProtoFieldString(3, "8662"),
		hongguoProtoFieldString(4, deviceID),
		hongguoProtoFieldString(5, hongguoMultiProcessNonce),
		hongguoProtoFieldString(6, "!noperm!"),
		hongguoProtoFieldSint(7, -888888),
		hongguoProtoFieldSint(8, -888888),
		hongguoProtoFieldSint(9, 3),
		hongguoProtoFieldSint(10, -888888),
		hongguoProtoFieldString(11, "!notset!"),
		hongguoProtoFieldString(12, "Asia/Shanghai,8"),
		hongguoProtoFieldString(13, "zh_CN"),
		hongguoProtoFieldSint(14, 4),
		hongguoProtoFieldFloat(16, 0),
		hongguoProtoFieldFloat(17, 0),
		hongguoProtoFieldFloat(18, 0),
		hongguoProtoFieldFloat(19, 0),
		hongguoProtoFieldFloat(20, 0),
		hongguoProtoFieldFloat(21, 0),
		hongguoProtoFieldString(22, "16"),
		hongguoProtoFieldSint(23, 41),
		hongguoProtoFieldSint(24, 36),
		hongguoProtoFieldSint(25, int64(hongguoMultiProcessStarted)),
		hongguoProtoFieldSint(26, int64(hongguoMultiProcessStarted)),
		hongguoProtoFieldSint(27, int64(hongguoMultiProcessStarted)),
		hongguoProtoFieldSint(28, int64(hongguoMultiProcessStarted)),
		hongguoProtoFieldSint(29, -1),
		hongguoProtoFieldString(30, "25053RT47C"),
		hongguoProtoFieldString(31, "Redmi"),
		hongguoProtoFieldString(32, "25053RT47C"),
		hongguoProtoFieldString(33, "25053RT47C"),
		hongguoProtoFieldString(34, "Xiaomi"),
		hongguoProtoFieldString(35, "Redmi"),
		hongguoProtoFieldString(36, "Redmi"),
		hongguoProtoFieldSint(38, 31),
	)
}

var hongguoSBOX, _ = base64.StdEncoding.DecodeString("+n0Ia5xZs0sEXznQOEqRmQBnpiCf9U2Ccybu3xhmgzOAAxn72f6uqqmwUsYL83klTni0NqxdGieeiNu9PGPsSRXBMB/cuFbUbM3KCUPINaPvHvSW0vwOcnuUhNHqRVpiAj/TEoE0K91+5ijypUYTATsh9mE3KSoN7Yyvv51cuyR2D3XkU4nhmI2xmmVwT1RMWKtub4sjxAcRDLrPoKSO2AU9FLLadMPX577Wf95IFj6FkKFVt3dCIsmGUC4X+WQxLJvxbRxEaOPpqJOXyzJX6+Vxaq3AzMfF/WAdoi1Hp+JRaV56zgpBtpWP97mH4DoGEIq1+FvV8LyS/3wvwugbQOwb2r26mJEMsiuDQTRn+wrYdrVGBVlhI3WQhyrjUBVMrLF5667llUcEaPCGPVGLD8qO5LlO8hKCvA7V9+8oJc9bXelqVQLhM76T5/WtnT45JKji+hdX0HoNCDDWuKON/QeaxB5uImSX0h2wv0VmP2zd2yeApxHcpsVS+MC2yFwAc2B7oBkTqsk1SEvTpM2fmfMQREBUfin0Bh+iq6EvPPavhWI2IX9e3yAas7Tm/3KEj2UmlFp36kN4x0rMLBRrxuh0U/zUHM4xcAMYjJY4MonxOl/X+alpt2M3WMI7w3HLnpIBigtNiJu7T21v4P6lSd5WFgntnMEt7oF92XzRLkJbTcGmXepE/UVOG6E/0YnhfS+q26utWcuxzpooyeD2cDlK1/8w9d28VzsRjbLuALbmGlp8+d7EzS6Au7lMpZ+ECMZvQmzwJ+eLOpxR+2chdUExp8ogQyq3v9l68rX4jCwjg0+PYKAEEzcU4wHFY2ZcdIHfWL1okD3SszT0GZMyKdZJrg1L2AeerB4tC0C4crp2EHGo5FYdSP7lwkeR2ocmnR+Ia8CYviUJlzOjhRZef9xuVOn3qcjow3fQgivsAmKKkg4+sA8F8/GWeDiGNhg8JM8KtFPMYWWkx5TVFX5t73kiNRJqjlIGVXtGZFCV4gzt0xcDopuZ6xz8r9Rzafpf9ywev8jh8592gHFIqpStZPuJxmDDMrNN0uBE3V+oscdoIzTJbRJ/t+sVvqnReJOgDJKk10fjisJwqyZBmnmn2BSFj8BvVtCMEbkuPOKdzw7eA11GPs04Qw8zWtkaZWwiO/wwpojqN6K0jY5RnNZA7vn4hPSul+nKCkVnVwQvg1zVxcSCtqORmB9KrJaBbssbCQivGJVJfVTt+hYxOtq4ZvWl8f4QAQZ0zGPffCgl9s6yT4vlvIdpu4YhBwA25wtQWZsc6GJYGWHyvSdeuh3mmUI9DSq13Fsp8C1MU3tqc04/df9LoTUXVXI5INOw/e8C7Hd+5CvbkMEFnnrUUmskEw==")

func hongguoXorBytes(a, b []byte) []byte {
	out := make([]byte, len(a))
	for i := range a {
		out[i] = a[i] ^ b[i%len(b)]
	}
	return out
}

type hongguoAesV3 struct {
	box   []byte
	con   [4]int
	con2  [4]int
	order [16]int
	keys  [12][4]byte
}

func newHongguoAesV3(key []byte, khronos uint32) *hongguoAesV3 {
	wordSize := int(khronos & 3)
	box := hongguoSBOX[wordSize*256 : (wordSize+1)*256]
	conSets := [4][4]int{{1, 0, 2, 3}, {1, 3, 0, 2}, {0, 1, 3, 2}, {1, 0, 2, 3}}
	con2Sets := [4][4]int{{1, 0, 2, 3}, {2, 0, 3, 1}, {0, 1, 3, 2}, {1, 0, 2, 3}}
	orderSets := [4][16]int{
		{0, 9, 14, 11, 4, 13, 2, 7, 8, 1, 6, 15, 12, 5, 10, 3},
		{0, 9, 14, 15, 4, 13, 2, 7, 8, 1, 6, 3, 12, 5, 10, 11},
		{0, 9, 14, 7, 4, 13, 2, 11, 8, 1, 6, 3, 12, 5, 10, 15},
		{0, 9, 14, 11, 4, 13, 2, 7, 8, 1, 6, 15, 12, 5, 10, 3},
	}
	initWords := [4]uint32{0xca025ddc, 0x823dc546, 0xc9420583, 0xc298225f}
	v := &hongguoAesV3{box: box, con: conSets[wordSize], con2: con2Sets[wordSize], order: orderSets[wordSize]}
	initial := make([]byte, 16)
	for i := 0; i < 4; i++ {
		binary.LittleEndian.PutUint32(initial[i*4:], initWords[wordSize])
	}
	mk := hongguoXorBytes(initial, key)
	expanded := make([]byte, 48)
	copy(expanded, mk)
	rounds := 8
	init := initWords[wordSize]
	for i := 4; i < 12; i++ {
		at := 4 * (i - 1)
		a, b, c, d := expanded[at], expanded[at+1], expanded[at+2], expanded[at+3]
		if i%4 == 0 {
			t := (uint32(init>>uint(rounds&24)) ^ uint32(box[b])) & 255
			b = box[c]
			c = box[d]
			d = box[a]
			a = byte(t)
		}
		rounds += 2
		expanded[at+4] = a ^ expanded[at-12]
		expanded[at+5] = b ^ expanded[at-11]
		expanded[at+6] = c ^ expanded[at-10]
		expanded[at+7] = d ^ expanded[at-9]
	}
	for i := 0; i < 12; i++ {
		copy(v.keys[i][:], expanded[i*4:i*4+4])
	}
	return v
}

func (v *hongguoAesV3) add(rows *[4][4]byte, keys [4][4]byte) {
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			rows[i][j] ^= keys[i][j]
		}
	}
}

func (v *hongguoAesV3) addCon(rows *[4][4]byte, keys [][4]byte) {
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			rows[i][j] ^= keys[i][v.con2[j]]
		}
	}
}

func (v *hongguoAesV3) sub(rows *[4][4]byte) {
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			rows[i][j] = v.box[rows[i][j]]
		}
	}
	old := *rows
	for i := 0; i < 4; i++ {
		rows[i] = old[v.con2[i]]
	}
}

func (v *hongguoAesV3) shift(rows *[4][4]byte) {
	var b [16]byte
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			b[i*4+j] = rows[i][j]
		}
	}
	for i := 0; i < 16; i++ {
		rows[i/4][i%4] = b[v.order[i]]
	}
}

func (v *hongguoAesV3) shiftCon(rows *[4][4]byte) {
	old := *rows
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			rows[i][j] = old[i][v.con2[j]]
		}
	}
}

func hongguoAesMix(rows *[4][4]byte) {
	xt := func(x byte) byte {
		doubled := uint16(x) << 1
		if x&128 != 0 {
			doubled ^= 0x1b
		}
		return byte(doubled)
	}
	for i := 0; i < 4; i++ {
		t := rows[0][i] ^ rows[1][i] ^ rows[2][i] ^ rows[3][i]
		u := rows[0][i]
		rows[0][i] ^= t ^ xt(rows[0][i]^rows[1][i])
		rows[1][i] ^= t ^ xt(rows[1][i]^rows[2][i])
		rows[2][i] ^= t ^ xt(rows[2][i]^rows[3][i])
		rows[3][i] ^= t ^ xt(rows[3][i]^u)
	}
}

func (v *hongguoAesV3) block(value []byte) []byte {
	var rows [4][4]byte
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			rows[i][j] = value[i*4+j]
		}
	}
	v.addCon(&rows, v.keys[0:4])
	for i := 1; i < 3; i++ {
		v.sub(&rows)
		v.shift(&rows)
		if i == 1 {
			v.shiftCon(&rows)
			hongguoAesMix(&rows)
		}
		v.addCon(&rows, v.keys[i*4:(i+1)*4])
	}
	var final [4][4]byte
	copy(final[:], v.keys[4:8])
	v.add(&rows, final)
	out := make([]byte, 16)
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			out[i*4+j] = rows[i][j]
		}
	}
	return out
}

func (v *hongguoAesV3) encrypt(data, iv []byte) []byte {
	source := append([]byte{}, data...)
	plaintext := make([]byte, 32)
	for i := 0; i < 31; i++ {
		at := i * 8
		if at+7 >= len(source) {
			break
		}
		n0 := (source[at] >> 4) & 2
		n1 := n0 | (source[at+1] & 64)
		n2 := n1 | ((source[at+2] >> 2) & 1)
		n3 := n2 | ((source[at+3] << 3) & 128)
		n4 := n3 | ((source[at+4] >> 1) & 4)
		n5 := n4 | ((source[at+5] << 3) & 16)
		n6 := n5 | ((source[at+6] << 5) & 32)
		plaintext[i] = n6 | ((source[at+7] >> 4) & 8)
	}
	plaintext[31] = 1
	var blocks []byte
	previous := append([]byte{}, iv...)
	for i := 0; i < len(plaintext); i += 16 {
		value := v.block(hongguoXorBytes(plaintext[i:i+16], previous))
		blocks = append(blocks, value...)
		previous = value
	}
	key := blocks
	out := append([]byte{}, source...)
	for i := 0; i < 31 && i*8+7 < len(out); i++ {
		at := i * 8
		k := key[i]
		out[at] &= 0xdf
		out[at] |= (k << 4) & 32
		out[at+1] &= 0xbf
		out[at+1] |= k & 64
		out[at+2] &= 0xfb
		out[at+2] |= (k << 2) & 4
		out[at+3] &= 0xef
		out[at+3] |= (k >> 3) & 16
		out[at+4] &= 0xf7
		out[at+4] |= (k + k) & 8
		out[at+5] &= 0xfd
		out[at+5] |= (k >> 3) & 2
		out[at+6] &= 0xfe
		out[at+6] |= (k >> 5) & 1
		out[at+7] &= 0x7f
		out[at+7] |= (k << 4) & 128
	}
	return append([]byte{key[len(key)-1]}, out...)
}

func hongguoXmxor(data, key []byte) []byte {
	encoded := make([]byte, len(data))
	for i := 0; i < len(data); i++ {
		at := (i * 4) & 28
		d0 := key[at]
		d1 := key[at+1]
		d2 := int32(((int(data[i]) << 4) | (int(data[i]) >> 4)) & 255)
		d2 += int32(d0)
		d2 = ^d2 ^ int32(d1)
		d2 = int32((uint32(d2&255)<<3 | (uint32(d2)&255)>>5) & 255)
		d2 = (d2 + int32(d1)) & 255
		d2 = (d2 ^ int32(d0)) & 255
		encoded[len(data)-i-1] = byte(uint32(^d2) & 255)
	}
	last := encoded[len(encoded)-1] ^ encoded[len(encoded)-2]
	first := encoded[0]
	encoded[0] = byte((uint32(^last) + uint32(first)) & 255)
	encoded[1] = byte((uint32(encoded[0]^encoded[len(encoded)-1]^254) + uint32(encoded[1])) & 255)
	rot := func(v byte) byte { return (v << 3) | (v >> 5) }
	encoded[2] = byte((uint32(encoded[2]) + uint32((last-first)^rot(encoded[1])^2)) & 255)
	for i := 0; i < len(encoded)-4; i++ {
		temp := uint32(rot(encoded[i+2])) ^ uint32(encoded[i+1]) ^ uint32(i+3)
		encoded[i+3] = byte((uint32(^int32(temp)) + uint32(encoded[i+3])) & 255)
	}
	encoded[len(encoded)-1] ^= encoded[len(encoded)-2]
	sum := 0
	for i := 0; i < len(encoded)-1; i++ {
		sum += int(encoded[i+1])
	}
	encoded[0] = byte((uint32(encoded[0]^encoded[1]) + uint32(sum)) & 255)
	return encoded
}

func hongguoKeyHash(signKey []byte, random uint32) ([]byte, []byte) {
	input := append(append(append([]byte{}, signKey...), hongguoLe32(random)...), signKey...)
	hash := hongguoMultiSM3(input)
	d1 := (random >> 16) & 255
	d2 := ((d1 << 11) | (random >> 24)) ^ (d1 >> 5) ^ d1
	d2 = ^d2
	return hash, hongguoLe32(d2)
}

func hongguoHelios(khronos uint32) string {
	random := hongguoMultiRandomUint32()
	data := append(hongguoLe32(random), []byte("8662")...)
	digest := md5.Sum(data)
	ascii := []byte(hex.EncodeToString(digest[:]))
	words := make([]uint64, 4)
	for i := 0; i < 4; i++ {
		words[i] = binary.LittleEndian.Uint64(ascii[i*8:])
	}
	table := []uint64{words[0]}
	b0 := words[0]
	b8 := words[1]
	remaining := append([]uint64{}, words[2:]...)
	for i := 0; i < 34; i++ {
		x8 := ror64(b8, 8) + b0
		x8 = x8 ^ uint64(i)
		remaining = append(remaining, x8)
		x8 = x8 ^ ror64(b0, 61)
		table = append(table, x8)
		b0 = x8
		b8 = remaining[0]
		remaining = remaining[1:]
	}
	text := []byte(fmt.Sprintf("%d-1588093228-8662", khronos))
	padded := (len(text) + 1 + 15) / 16 * 16
	if padded == 0 {
		padded = 16
	}
	pad := make([]byte, padded)
	for i := range pad {
		pad[i] = byte(16 - len(text)%16)
	}
	copy(pad, text)
	var output []byte
	for at := 0; at < len(pad); at += 16 {
		a := binary.LittleEndian.Uint64(pad[at:])
		b := binary.LittleEndian.Uint64(pad[at+8:])
		for i := 0; i < 34; i++ {
			b = table[i] ^ (a + ror64(b, 8))
			a = b ^ ror64(a, 61)
		}
		block := make([]byte, 16)
		binary.LittleEndian.PutUint64(block, a)
		binary.LittleEndian.PutUint64(block[8:], b)
		output = append(output, block...)
	}
	return base64.StdEncoding.EncodeToString(append(hongguoLe32(random), output...))
}

func hongguoPythonUrlEncode(pairs [][2]string) string {
	var builder strings.Builder
	encode := func(value string) {
		for i := 0; i < len(value); i++ {
			c := value[i]
			keep := c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' ||
				c == '-' || c == '_' || c == '.' || c == '~' || c == '*' || c == '!' || c == '\'' || c == '(' || c == ')'
			if keep {
				builder.WriteByte(c)
				continue
			}
			builder.WriteString(fmt.Sprintf("%%%02X", c))
		}
	}
	for _, pair := range pairs {
		if builder.Len() > 0 {
			builder.WriteByte('&')
		}
		encode(pair[0])
		builder.WriteByte('=')
		encode(pair[1])
	}
	return builder.String()
}

var hongguoMultiDevicePairsTemplate = [][2]string{
	{"ac", "wifi"}, {"channel", "update_64"}, {"aid", "8662"}, {"app_name", "novelread"},
	{"version_code", "71332"}, {"version_name", "7.1.3.32"}, {"device_platform", "android"}, {"os", "android"},
	{"ssmix", "a"}, {"device_type", "25053RT47C"}, {"device_brand", "Redmi"}, {"language", "zh"}, {"os_api", "36"},
	{"os_version", "16"}, {"manifest_version_code", "71332"}, {"resolution", "1280*2772"}, {"dpi", "520"},
	{"update_version_code", "71332"}, {"host_abi", "arm64-v8a"}, {"dragon_device_type", "phone"}, {"pv_player", "71332"},
	{"compliance_status", "0"}, {"need_personal_recommend", "1"}, {"player_so_load", "1"}, {"is_android_pad_screen", "0"},
}

func hongguoSignKey() []byte {
	raw, _ := hex.DecodeString("8ebdfa3806ecc5cee79423e6029ed82540bc2218bb7eaef71cb691f7aa8aa2f5")
	return raw
}

func hongguoBuildMedusa(fullURL string, body []byte, khronos uint32, deviceID, versionName string) string {
	bodyMd5 := md5.Sum(body)
	query := fullURL[strings.Index(fullURL, "?")+1:]
	querySm3 := hongguoMultiSM3([]byte(query))
	ts := hongguoLe32(khronos)
	queryBodyTs := hongguoHashF13(querySm3, bodyMd5[:], ts, khronos)
	nested := hongguoProto(
		hongguoProtoFieldSint(1, 111),
		hongguoProtoFieldSint(2, 10),
		hongguoProtoFieldSint(3, 694367),
		hongguoProtoFieldSint(5, 586952199),
	)
	messageRand := hongguoMultiRandomUint32()
	envLaunch := int64(hongguoMultiRandomInt(21) + 100)
	envPid := int64(hongguoMultiRandomInt(2000) + 10001)
	env := hongguoProto(
		hongguoProtoFieldSint(1, envLaunch),
		hongguoProtoFieldSint(2, 146331399),
		hongguoProtoFieldSint(3, 146331396),
		hongguoProtoFieldSint(5, 7),
		hongguoProtoFieldString(6, "v04.06.04.03-bugfix"),
		hongguoProtoFieldSint(7, envPid),
		hongguoProtoFieldBytes(12, hongguoDeviceProto(deviceID)),
		hongguoProtoFieldBytes(13, hongguoProto(hongguoProtoFieldSint(1, hongguoMultiNowUnix()), hongguoProtoFieldSint(2, -2), hongguoProtoFieldSint(4, 200))),
		hongguoProtoFieldString(14, versionName),
	)
	queryHash := hongguoMultiSM3(append(append(append([]byte{}, query...), bodyMd5[:]...), []byte("none")...))
	message := hongguoProto(
		hongguoProtoFieldBytes(1, mustHex("f7e85ffad7d7dc3bd62ac87057cf6118")),
		hongguoProtoFieldSint(2, 3),
		hongguoProtoFieldSint(3, int64(messageRand)),
		hongguoProtoFieldString(4, "8662"),
		hongguoProtoFieldString(5, deviceID),
		hongguoProtoFieldString(6, "1588093228"),
		hongguoProtoFieldString(7, versionName),
		hongguoProtoFieldString(8, "v04.06.04-ml-android"),
		hongguoProtoFieldSint(9, 67503104),
		hongguoProtoFieldBytes(10, mustHex("4001000000000000")),
		hongguoProtoFieldSint(12, int64(khronos)),
		hongguoProtoFieldBytes(13, queryBodyTs),
		hongguoProtoFieldBytes(14, querySm3[:6]),
		hongguoProtoFieldBytes(15, nested),
		hongguoProtoFieldString(16, hongguoMultiProcessNonce),
		hongguoProtoFieldSint(17, int64(khronos)),
		hongguoProtoFieldBytes(19, queryHash),
		hongguoProtoFieldString(20, "none"),
		hongguoProtoFieldSint(21, 312),
		hongguoProtoFieldBytes(23, env),
		hongguoProtoFieldString(24, `{"cmr":16777216,"cmr2":16777216,"un_h":1879194040,"vpn":0,"kd":0,"fkd":3672518972,"pd":-1872573247,"dyn":"","do":0,"tk":true}`),
	)
	random := hongguoMultiRandomUint32()
	hash, seed := hongguoKeyHash(hongguoSignKey(), random)
	transformed := hongguoXmxor(message, hash)
	prefix8 := mustHex("4001000000000000")
	transformed = append(append([]byte{}, prefix8...), transformed...)
	for i, j := 0, len(transformed)-1; i < j; i, j = i+1, j-1 {
		transformed[i], transformed[j] = transformed[j], transformed[i]
	}
	for i := range transformed {
		transformed[i] ^= seed[3-i%4]
	}
	check := uint32(querySm3[0]&63)<<14 | 0x18000001 | uint32(queryBodyTs[0]&63)<<8
	xmRand := hongguoMultiRandomUint32()
	packed := append([]byte{0x35}, hongguoLe32(xmRand)...)
	packed = append(packed, hongguoLe32(check)...)
	packed = append(packed, transformed...)
	packed = append(packed, byte(random>>16), byte(random>>24))
	aes := newHongguoAesV3(mustHex("f1593376766ea98d34f31b057a9d5be4"), khronos)
	encrypted := aes.encrypt(packed, mustHex("1fe109a4125283f418de9e051a969e12"))
	version := mustHex("03000000f7e85ffad7d7dc3bd62ac87057cf6118")
	prefix := make([]byte, 20)
	for i := 0; i < 20; i += 4 {
		binary.LittleEndian.PutUint32(prefix[i:], binary.LittleEndian.Uint32(version[i:])^khronos)
	}
	final := append(append(prefix, byte(random&255), byte(random>>8)&255, 0, 1), encrypted...)
	return base64.StdEncoding.EncodeToString(final)
}

func mustHex(value string) []byte {
	raw, err := hex.DecodeString(value)
	if err != nil {
		panic(err)
	}
	return raw
}

func hongguoMultiBranchOf(fullURL string, body []byte, khronos uint32) int {
	bodyMd5 := md5.Sum(body)
	query := fullURL[strings.Index(fullURL, "?")+1:]
	q := hongguoMultiSM3([]byte(query))
	iv := hongguoGetIv(hongguoGetIv(hongguoGetIv(0x20230928, q), bodyMd5[:]), hongguoLe32(khronos))
	low := int(iv & 15)
	return low - ((low*171)>>9)*3
}

type hongguoMultiMediaRow struct {
	item   map[string]any
	codec  string
	rank   int
	pixels int
	width  int
	height int
}

func hongguoMultiCleanURL(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\\/", "/")
	value = strings.ReplaceAll(value, "\\u0026", "&")
	value = strings.ReplaceAll(value, "&amp;", "&")
	return value
}

func hongguoMultiCheckedURL(value string, domains []string) (string, error) {
	if !strings.HasPrefix(value, "https://") {
		return "", errors.New("片源地址格式无效")
	}
	host := value
	if idx := strings.Index(value[8:], "/"); idx >= 0 {
		host = value[8 : 8+idx]
	}
	if strings.Contains(host, "@") {
		return "", errors.New("片源地址格式无效")
	}
	for _, domain := range domains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return value, nil
		}
	}
	return "", errors.New("片源返回了不受支持的服务地址")
}

func hongguoDeriveContentKey(value string) ([]byte, error) {
	normalized := strings.TrimSpace(value)
	normalized = strings.ReplaceAll(normalized, "-", "+")
	normalized = strings.ReplaceAll(normalized, "_", "/")
	raw, err := base64.StdEncoding.DecodeString(normalized)
	if err != nil {
		if raw, err = base64.RawStdEncoding.DecodeString(normalized); err != nil {
			return nil, errors.New("媒体密钥编码无效")
		}
	}
	if len(raw) < 3 {
		return nil, errors.New("媒体密钥编码无效")
	}
	v8 := len(raw) - int(raw[0]^raw[1]^raw[2]) + 47
	if v8 > len(raw)-1 {
		v8 = len(raw) - 1
	}
	if v8 < 33 {
		return nil, errors.New("媒体密钥长度无效")
	}
	work := append([]byte{}, raw[1:1+v8]...)
	a, b := byte(85), byte(246)
	for i := 0; i < v8; i++ {
		old := work[i]
		var previous byte
		if i&1 != 0 {
			previous = a
			a = old
		} else {
			previous = b
			b = old
		}
		pop := 0
		for remaining := i; remaining != 0; remaining &= remaining - 1 {
			pop++
		}
		work[i] = byte(-21 - pop + int(previous^old))
	}
	hexKey := string(work[1:33])
	if len(hexKey) != 32 {
		return nil, errors.New("媒体密钥格式无效")
	}
	for _, c := range hexKey {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return nil, errors.New("媒体密钥格式无效")
		}
	}
	return hex.DecodeString(strings.ToLower(hexKey))
}

func hongguoDecryptSpadeURL(value string, seed []byte) (string, error) {
	encoded := hongguoMultiCleanURL(value)
	if encoded == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		if raw, err = base64.RawStdEncoding.DecodeString(encoded); err != nil {
			return "", errors.New("加密片源地址编码无效")
		}
	}
	if len(raw) < 5 || raw[0] != 0xa8 || raw[2] != 0x01 || raw[3] != 0x00 {
		return "", errors.New("加密片源地址头格式无效")
	}
	cipherLength := (len(raw) - 4) / 16 * 16
	if cipherLength == 0 {
		return "", errors.New("加密片源地址缺少完整分组")
	}
	constants, _ := hex.DecodeString("4dd4c2e6b83162090e52b3c7a6733ba41cb2462b829ab58a196b39db57177524f49baf7f08e8d68d26a72e37c1a95a2f1f05a51892aef2949732b62a38aadd58")
	h1 := sha512.Sum512(seed)
	h2 := sha512.Sum512(append(append([]byte{}, h1[:]...), constants...))
	block, err := aes.NewCipher(h2[:16])
	if err != nil {
		return "", err
	}
	plain := make([]byte, cipherLength)
	cipher.NewCBCDecrypter(block, h2[16:32]).CryptBlocks(plain, raw[4:4+cipherLength])
	if len(plain) > 0 {
		padding := int(plain[len(plain)-1])
		if padding >= 1 && padding <= 16 && padding <= len(plain) {
			plain = plain[:len(plain)-padding]
		}
	}
	for len(plain) > 0 && plain[len(plain)-1] == 0 {
		plain = plain[:len(plain)-1]
	}
	return string(plain), nil
}

func hongguoMultiFetchJSON(ctx context.Context, client *http.Client, fullURL string, method string, headers map[string]string, body []byte) (map[string]any, error) {
	current := fullURL
	for redirects := 0; redirects < 4; redirects++ {
		request, err := http.NewRequestWithContext(ctx, method, current, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		for key, value := range headers {
			request.Header.Set(key, value)
		}
		response, err := client.Do(request)
		if err != nil {
			return nil, err
		}
		if response.StatusCode >= 301 && response.StatusCode <= 308 {
			location := response.Header.Get("Location")
			response.Body.Close()
			if method == http.MethodPost || location == "" {
				return nil, errors.New("接口发生不支持的跳转")
			}
			if strings.HasPrefix(location, "/") {
				parsed, _ := url.Parse(current)
				location = parsed.Scheme + "://" + parsed.Host + location
			}
			current = location
			continue
		}
		content, readErr := io.ReadAll(io.LimitReader(response.Body, 2<<20))
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("红果 App 接口 HTTP %d", response.StatusCode)
		}
		if readErr != nil {
			return nil, readErr
		}
		var result map[string]any
		decoder := json.NewDecoder(bytes.NewReader(content))
		decoder.UseNumber()
		if err := decoder.Decode(&result); err != nil || result == nil {
			return nil, errors.New("接口未返回有效数据")
		}
		return result, nil
	}
	return nil, errors.New("接口跳转次数过多")
}

func hongguoMultiParseFallback(value any) string {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if strings.HasPrefix(trimmed, "{") {
			var parsed map[string]any
			if json.Unmarshal([]byte(trimmed), &parsed) == nil {
				if inner, ok := parsed["fallback_api"].(string); ok {
					return inner
				}
			}
		}
		return trimmed
	case []any:
		if len(typed) > 0 {
			return strings.TrimSpace(fmt.Sprint(typed[0]))
		}
	case map[string]any:
		if inner, ok := typed["fallback_api"].(string); ok {
			return inner
		}
	}
	return ""
}

func (downloader *Downloader) resolveHongguoMultiMedia(ctx context.Context, videoID string) (providerMedia, error) {
	if !hongguoNumericID.MatchString(videoID) {
		return providerMedia{}, errors.New("红果视频 ID 无效")
	}
	payload, err := json.Marshal(map[string]any{
		"biz_param": map[string]any{"detail_page_version": 0, "device_level": 3, "disable_digg_stat": false,
			"need_all_video_definition": true, "need_mp4_align": false, "use_os_player": false, "use_server_dns": false,
			"video_platform": 1024},
		"mixed_video_id_map": map[string]any{"1004": []string{videoID}},
	})
	if err != nil {
		return providerMedia{}, err
	}
	client := downloader.client
	response, err := downloader.fetchHongguoRegisteredModel(ctx, payload)
	if err != nil {
		return providerMedia{}, err
	}
	code := firstNonEmpty(mapString(response, "code", "Code"), "0")
	if code != "0" {
		return providerMedia{}, errors.New("红果 App 接口暂未提供本集播放信息")
	}
	data, _ := response["data"].(map[string]any)
	entry, _ := data[videoID].(map[string]any)
	if entry == nil {
		return providerMedia{}, errors.New("红果 App 接口未返回所选分集")
	}
	modelRaw, ok := entry["video_model"]
	if !ok {
		return providerMedia{}, errors.New("红果 App 接口未返回播放模型")
	}
	var model map[string]any
	switch typed := modelRaw.(type) {
	case string:
		decoder := json.NewDecoder(strings.NewReader(typed))
		decoder.UseNumber()
		if err := decoder.Decode(&model); err != nil || model == nil {
			return providerMedia{}, errors.New("红果 App 播放模型格式异常")
		}
	case map[string]any:
		model = typed
	default:
		return providerMedia{}, errors.New("红果 App 播放模型格式异常")
	}
	inline := len(hongguoMultiMediaRows(model["video_list"])) > 0
	infoData := model
	if !inline {
		fallback := hongguoMultiParseFallback(model["fallback_api"])
		if fallback == "" {
			return providerMedia{}, errors.New("红果 App 接口未返回本集片源地址")
		}
		if _, err := hongguoMultiCheckedURL(fallback, []string{"fqnovel.com", "snssdk.com"}); err != nil {
			return providerMedia{}, err
		}
		outer, err := hongguoMultiFetchJSON(ctx, client, fallback, http.MethodGet, map[string]string{"User-Agent": hongguoMultiUserAgent}, nil)
		if err != nil {
			return providerMedia{}, err
		}
		if code := firstNonEmpty(mapString(outer, "code", "Code"), "0"); code != "0" {
			return providerMedia{}, errors.New("红果片源接口暂未提供本集播放信息")
		}
		infoData = nestedMap(nestedMap(outer, "video_info"), "data")
		if infoData == nil {
			return providerMedia{}, errors.New("红果 App 接口未返回本集片源模型")
		}
	}
	keySeed := mapString(infoData, "key_seed")
	var seedBytes []byte
	if keySeed != "" {
		if seedBytes, err = base64.StdEncoding.DecodeString(keySeed); err != nil {
			return providerMedia{}, errors.New("片源密钥编码无效")
		}
	}
	rows := hongguoMultiMediaRows(infoData["video_list"])
	if len(rows) == 0 {
		return providerMedia{}, errors.New("红果 App 未返回兼容的媒体，已跳过不支持的编码")
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].rank != rows[j].rank {
			return rows[i].rank < rows[j].rank
		}
		return rows[i].pixels > rows[j].pixels
	})
	var variants []providerMedia
	var selected providerMedia
	var lastErr error
	seen := map[string]bool{}
	for _, row := range rows {
		spade := mapString(row.item, "spade_a")
		if spade == "" {
			spade = mapString(nestedMap(row.item, "encrypt_info"), "spade_a")
		}
		if inline && spade == "" && !hongguoExplicitlyUnencrypted(row.item) {
			lastErr = errors.New("本集片源缺少有效解密信息")
			continue
		}
		var key []byte
		if spade != "" {
			key, err = hongguoDeriveContentKey(spade)
			if err != nil {
				lastErr = err
				continue
			}
		}
		candidates := []string{mapString(row.item, "main_url"), mapString(row.item, "backup_url")}
		if backupList, ok := row.item["backup_url_list"].([]any); ok {
			for _, backup := range backupList {
				candidates = append(candidates, strings.TrimSpace(fmt.Sprint(backup)))
			}
		}
		for _, candidate := range candidates {
			if candidate == "" {
				continue
			}
			var media string
			if len(seedBytes) > 0 {
				if media, err = hongguoDecryptSpadeURL(candidate, seedBytes); err != nil {
					lastErr = err
					continue
				}
			} else {
				media = hongguoMultiCleanURL(candidate)
			}
			checked, err := hongguoMultiCheckedURL(media, []string{"qznovelvod.com", "fqnovelvod.com", "douyinvod.com"})
			if err != nil {
				lastErr = err
				continue
			}
			identity := checked + "\x00" + hex.EncodeToString(key)
			if seen[identity] {
				continue
			}
			seen[identity] = true
			variant := providerMedia{Direct: true, URL: checked, Referer: "https://novel.snssdk.com/", CENCKey: key, Quality: row.pixels}
			variants = append(variants, variant)
			if selected.URL == "" {
				selected = variant
			}
		}
	}
	if selected.URL == "" {
		if lastErr != nil {
			return providerMedia{}, lastErr
		}
		return providerMedia{}, errors.New("本集片源地址或密钥无法解析")
	}
	selected.Direct = true
	selected.Variants = variants
	return selected, nil
}

func hongguoMultiMediaRows(list any) []hongguoMultiMediaRow {
	var raw []any
	switch typed := list.(type) {
	case []any:
		raw = typed
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			raw = append(raw, typed[key])
		}
	}
	var rows []hongguoMultiMediaRow
	for _, entry := range raw {
		item, _ := entry.(map[string]any)
		if item == nil {
			continue
		}
		if mapString(item, "main_url") == "" && mapString(item, "backup_url") == "" && item["backup_url_list"] == nil {
			continue
		}
		meta := nestedMap(item, "video_meta")
		codec := strings.ToLower(firstNonEmpty(mapString(item, "codec_type"), mapString(meta, "codec_type")))
		rank := -1
		switch codec {
		case "h264", "avc", "avc1":
			rank = 0
		case "hevc", "h265", "hvc1", "hev1", "bytevc1":
			rank = 1
		default:
			continue
		}
		qualityText := firstNonEmpty(mapString(item, "quality_desc"), mapString(item, "definition"), mapString(item, "height"), mapString(item, "vheight"), mapString(meta, "definition"))
		pixels := 0
		if match := hongguoQualityNumber.FindString(qualityText); match != "" {
			pixels, _ = strconv.Atoi(match)
		}
		width, height := 0, 0
		for _, pair := range [][2]string{{mapString(item, "vwidth"), mapString(item, "vheight")}, {mapString(item, "width"), mapString(item, "height")}, {mapString(meta, "vwidth"), mapString(meta, "vheight")}, {mapString(meta, "width"), mapString(meta, "height")}} {
			w, err1 := strconv.Atoi(pair[0])
			h, err2 := strconv.Atoi(pair[1])
			if err1 == nil && err2 == nil && w > 0 && h > 0 {
				width, height = w, h
				break
			}
		}
		if width > 0 && height > 0 {
			pixels = min(width, height)
		}
		rows = append(rows, hongguoMultiMediaRow{item: item, codec: codec, rank: rank, pixels: pixels, width: width, height: height})
	}
	return rows
}

func hongguoExplicitlyUnencrypted(item map[string]any) bool {
	value := nestedMap(item, "encrypt_info")["encrypt"]
	switch typed := value.(type) {
	case bool:
		return !typed
	case json.Number:
		return typed.String() == "0"
	case string:
		return typed == "0" || typed == "false"
	}
	return false
}
