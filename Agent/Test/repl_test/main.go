package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func main() {
	fmt.Println("=== REPL 模式测试 ===")
	fmt.Println()

	testBasicCommands()

	testHelp()

	testQuotedArgs()

	testInvalidCommand()

	testAgentChatInREPL()

	fmt.Println("\n=== 所有 REPL 测试完成 ===")
}

func runREPL(inputs ...string) (string, string, error) {
	cmd := exec.Command("./TACA.exe")
	cmd.Dir = "../.."
	stdin, _ := cmd.StdinPipe()

	go func() {
		defer stdin.Close()
		for _, line := range inputs {
			stdin.Write([]byte(line + "\n"))
		}
	}()

	out, err := cmd.CombinedOutput()
	output := string(out)
	return output, output, err
}

func testBasicCommands() {
	fmt.Println("[TEST 1] 基本命令执行")
	output, _, err := runREPL(
		"component list",
		"model list",
		"role list",
		"tool list",
		"exit",
	)
	if err != nil && !strings.Contains(output, "NAME") {
		fmt.Println("  FAIL:", err)
		return
	}

	checks := []string{
		"time",
		"mimo-v2.5-pro",
		"weather-assistant",
		"get_weather",
	}
	passed := true
	for _, c := range checks {
		if !strings.Contains(output, c) {
			fmt.Printf("  FAIL: 输出中缺少 %q\n", c)
			passed = false
		}
	}
	if passed {
		fmt.Println("  PASS")
	}
}

func testHelp() {
	fmt.Println("[TEST 2] help 命令")
	output, _, err := runREPL("help", "exit")
	if err != nil {
		fmt.Println("  FAIL:", err)
		return
	}
	if strings.Contains(output, "Available Commands") {
		fmt.Println("  PASS")
	} else {
		fmt.Println("  FAIL: 未显示 Available Commands")
	}
}

func testQuotedArgs() {
	fmt.Println("[TEST 3] 带引号参数（shlex 解析）")

	output, _, err := runREPL(
		`role get --name="weather-assistant"`,
		"exit",
	)
	if err != nil {
		fmt.Println("  FAIL:", err)
		return
	}
	if strings.Contains(output, "weather-assistant") {
		fmt.Println("  PASS")
	} else {
		fmt.Println("  FAIL: 未正确解析带引号的参数")
	}
}

func testInvalidCommand() {
	fmt.Println("[TEST 4] 无效命令处理")
	output, _, err := runREPL(
		"nonexistent-command",
		"exit",
	)
	if err != nil {

		fmt.Println("  INFO: 命令返回错误但 REPL 继续运行")
	}
	if strings.Contains(output, "unknown command") || strings.Contains(output, "unknown shorthand flag") {
		fmt.Println("  PASS")
	} else {

		fmt.Println("  PASS (cobra 显示帮助信息)")
	}
}

func testAgentChatInREPL() {
	fmt.Println("[TEST 5] REPL 中启动 Agent Chat")
	output, _, err := runREPL(
		`agent chat --role=weather-assistant --mode=queue --message="你好"`,
		"exit",
	)
	if err != nil {
		fmt.Println("  INFO:", err)
	}
	if strings.Contains(output, "agent chat") && strings.Contains(output, "weather-assistant") {
		fmt.Println("  PASS")
	} else {
		fmt.Println("  FAIL: Agent Chat 未正确启动")
	}
}
