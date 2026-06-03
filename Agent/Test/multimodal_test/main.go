package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/Lyoomu/TAC/Agent/internal/config"
	"github.com/Lyoomu/TAC/Agent/internal/crypto"
	"github.com/Lyoomu/TAC/Agent/internal/db"
	"github.com/Lyoomu/TAC/Agent/internal/model"
	"github.com/Lyoomu/TAC/Agent/internal/models"
	"github.com/Lyoomu/TAC/Agent/internal/models/llm"
	"github.com/Lyoomu/TAC/Agent/internal/repository"
)

func main() {
	fmt.Println("=== 多模态对话测试 ===")
	fmt.Println()

	mgr := initManager()
	if mgr == nil {
		return
	}

	picPath := createTestImage()
	if picPath == "" {
		fmt.Println("创建测试图片失败")
		return
	}
	fmt.Println("测试图片:", picPath)

	picName := "test_red.png"

	testImageChat(mgr, picName)

	testMultimodalChat(mgr, picName)

	fmt.Println("\n=== 多模态测试完成 ===")
}

func initManager() *llm.Client {
	cfg, err := config.Load("properties.yaml")
	if err != nil {
		fmt.Println("Config error:", err)
		return nil
	}

	dbConn, err := db.Open(cfg.WorkPath.DB)
	if err != nil {
		fmt.Println("DB error:", err)
		return nil
	}
	encrypter := crypto.New("this-is-a-test-key-for-development-only-32chars")
	modelRepo := repository.NewModelRepo(dbConn, encrypter)

	mList, _ := models.NewEngine(modelRepo).List()
	var defaultModel *model.Model
	if len(mList) > 0 {
		defaultModel, _ = modelRepo.Get(mList[0].Name)
	}
	if defaultModel == nil {
		fmt.Println("No model found")
		return nil
	}

	return llm.NewClient(defaultModel, cfg)
}

func createTestImage() string {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	red := color.RGBA{255, 0, 0, 255}
	for x := 0; x < 100; x++ {
		for y := 0; y < 100; y++ {
			img.Set(x, y, red)
		}
	}

	picDir := "source/pic"
	os.MkdirAll(picDir, 0755)
	path := filepath.Join(picDir, "test_red.png")
	f, err := os.Create(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		return ""
	}
	return path
}

func testImageChat(client *llm.Client, picPath string) {
	fmt.Println("\n[TEST 1] ChatWithImages - 发送图片")
	resp, err := client.ChatWithImages(
		"描述这张图片的颜色",
		[]string{picPath},
		nil,
	)
	if err != nil {
		if isAPINotSupported(err) {
			fmt.Printf("  SKIP - API 不支持图片输入: %v\n", err)
		} else {
			fmt.Printf("  FAIL: %v\n", err)
		}
		return
	}
	fmt.Printf("  PASS - 响应: %s\n", truncate(resp, 200))
}

func testMultimodalChat(client *llm.Client, picPath string) {
	fmt.Println("\n[TEST 2] ChatMultimodal - 文本+图片混合")
	resp, err := client.ChatMultimodal(
		"这是什么颜色？",
		[]llm.MediaFile{{Path: picPath, Type: "image"}},
		nil,
	)
	if err != nil {
		if isAPINotSupported(err) {
			fmt.Printf("  SKIP - API 不支持图片输入: %v\n", err)
		} else {
			fmt.Printf("  FAIL: %v\n", err)
		}
		return
	}
	fmt.Printf("  PASS - 响应: %s\n", truncate(resp, 200))
}

func isAPINotSupported(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "No endpoints found") || strings.Contains(s, "image input") || strings.Contains(s, "not supported")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
