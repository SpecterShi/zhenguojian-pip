package core

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestProbeDiagnosticFlow(t *testing.T) {
	if os.Getenv("HG_PROBE") == "" {
		t.Skip("probe disabled")
	}
	engine, err := newNativeEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d := engine.downloader
	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
	defer cancel()

	// 复刻诊断：目录第1页 → 第一个非VIP剧 → 第一个非VIP分集 → 取流 → 媒体字节
	page, err := engine.nativeCatalog(ctx, nativeInput{Source: "hongguo", Page: 1, Force: true})
	if err != nil {
		t.Fatalf("目录失败: %v", err)
	}
	drama := page.Items[0]
	for _, item := range page.Items {
		if item.VIP == nil || !*item.VIP {
			drama = item
			break
		}
	}
	fmt.Printf("选中剧: %s (%s) VIP=%v\n", drama.Title, drama.ID, drama.VIP != nil && *drama.VIP)

	_, id, _ := splitProviderDramaID(drama.ID)
	_, chapters, err := d.GetHuangguoChapters(ctx, "hongguo", id)
	if err != nil {
		t.Fatalf("分集失败: %v", err)
	}
	var chapter Chapter
	for _, item := range chapters {
		if !item.VIP {
			chapter = item
			break
		}
	}
	fmt.Printf("选中分集: %s vid=%s\n", chapter.Title, chapter.VideoURL)

	media, err := d.resolveProviderMedia(ctx, Task{DramaID: drama.ID, Chapter: chapter, Index: 1})
	if err != nil {
		t.Fatalf("取流失败: %v", err)
	}
	fmt.Printf("媒体URL=%.90s referer=%s key=%v\n", media.URL, media.Referer, media.CENCKey != nil)

	body, err := d.sourceProbeBytes(ctx, media.URL, media.Referer, false)
	fmt.Printf("媒体探测: %d bytes err=%v\n", len(body), err)
	if err == nil {
		fmt.Printf("head16=%x\n", body[:minInt(16, len(body))])
	}

	// 对照组：已知可播的剧
	known := Task{DramaID: "hongguo:7503745441910017049", Chapter: Chapter{VideoURL: "hongguo-cenc://7503780574541122622"}}
	media2, err := d.resolveProviderMedia(ctx, known)
	if err != nil {
		t.Fatalf("对照取流失败: %v", err)
	}
	body2, err2 := d.sourceProbeBytes(ctx, media2.URL, media2.Referer, false)
	fmt.Printf("对照剧: %d bytes err=%v\n", len(body2), err2)
	if err2 == nil {
		fmt.Printf("对照head16=%x\n", body2[:minInt(16, len(body2))])
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
