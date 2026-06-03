package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/Lyoomu/TAC/Agent/internal/agent"
	"github.com/Lyoomu/TAC/Agent/internal/component"
	"github.com/Lyoomu/TAC/Agent/internal/config"
	"github.com/Lyoomu/TAC/Agent/internal/crypto"
	"github.com/Lyoomu/TAC/Agent/internal/db"
	"github.com/Lyoomu/TAC/Agent/internal/models"
	"github.com/Lyoomu/TAC/Agent/internal/repository"
	"github.com/Lyoomu/TAC/Agent/internal/role"
	"github.com/Lyoomu/TAC/Agent/internal/tool"
)

func main() {
	mgr := initManager()
	if mgr == nil {
		return
	}

	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("测试 1: Reject 模式")
	fmt.Println(strings.Repeat("=", 60))
	testReject(mgr)

	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("测试 2: Interrupt 模式")
	fmt.Println(strings.Repeat("=", 60))
	testInterrupt(mgr)
}

func initManager() *agent.Manager {
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
	if err := db.MigrateUp(dbConn); err != nil {
		fmt.Println("Migrate error:", err)
		return nil
	}
	encrypter := crypto.New("this-is-a-test-key-for-development-only-32chars")

	componentRepo := repository.NewComponentRepo(dbConn)
	modelRepo := repository.NewModelRepo(dbConn, encrypter)
	roleRepo := repository.NewRoleRepo(dbConn)

	componentEngine := component.NewEngine(componentRepo)
	roleEngine := role.NewEngine(roleRepo, componentEngine)
	toolEngine := tool.NewEngine(dbConn, cfg.WorkPath.Tool)
	_ = toolEngine.Register()

	modelsEngine := models.NewEngine(modelRepo)
	return agent.NewManager(roleEngine, toolEngine, modelsEngine, cfg)
}

func testReject(mgr *agent.Manager) {
	output, err := mgr.RunAgent("weather-assistant", nil, "请写一段300字的自我介绍，越长越好", nil, agent.ModeReject)
	if err != nil {
		fmt.Println("RunAgent error:", err)
		return
	}
	defer output.Stop()

	rejected := make(chan struct{})

	go func() {
		for chunk := range output.StreamCh {
			fmt.Print(chunk)
		}
	}()

	go func() {
		for err := range output.ErrCh {
			fmt.Printf("\n[REJECT ERR] %v\n", err)
			if strings.Contains(err.Error(), "rejected") {
				close(rejected)
			}
		}
	}()

	time.Sleep(500 * time.Millisecond)
	fmt.Println("\n[TEST] 发送第二条消息（应该被拒绝）...")
	output.SendMessage("北京天气如何？", agent.ModeReject)

	select {
	case <-rejected:
		fmt.Println("[REJECT TEST] PASS - 消息被拒绝")
	case <-time.After(20 * time.Second):
		fmt.Println("[REJECT TEST] FAIL - 超时未收到拒绝")
	}
}

func testInterrupt(mgr *agent.Manager) {
	output, err := mgr.RunAgent("weather-assistant", nil, "请写一段300字的自我介绍，越长越好", nil, agent.ModeInterrupt)
	if err != nil {
		fmt.Println("RunAgent error:", err)
		return
	}
	defer output.Stop()

	interrupted := make(chan struct{})
	var interruptSeen bool

	go func() {
		for chunk := range output.StreamCh {
			fmt.Print(chunk)
			if strings.Contains(chunk, "[interrupt]") {
				if !interruptSeen {
					interruptSeen = true
					close(interrupted)
				}
			}
		}
	}()

	go func() {
		for err := range output.ErrCh {
			fmt.Printf("\n[INTERRUPT ERR] %v\n", err)
		}
	}()

	time.Sleep(2 * time.Second)
	fmt.Println("\n[TEST] 发送中断消息...")
	output.SendMessage("[interrupt] 停下，告诉我北京天气", agent.ModeInterrupt)

	select {
	case <-interrupted:
		fmt.Println("\n[INTERRUPT TEST] PASS - [interrupt] 标记出现")
		fmt.Println("[TEST] 继续等待 LLM 处理新消息...")

		time.Sleep(20 * time.Second)
	case <-time.After(30 * time.Second):
		fmt.Println("\n[INTERRUPT TEST] FAIL - 超时未收到中断标记")
	}
}
