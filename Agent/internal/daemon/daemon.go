package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"
)

func configDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".tac", "agent")
}

func pidPath() string  { return filepath.Join(configDir(), "pid") }
func portPath() string { return filepath.Join(configDir(), "port") }

func IsRunning() bool {
	pid, err := readPID()
	if err != nil {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

func ReadPort() (string, error) {
	data, err := os.ReadFile(portPath())
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func SaveState(pid int, port string) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(pidPath(), []byte(strconv.Itoa(pid)), 0644); err != nil {
		return err
	}
	return os.WriteFile(portPath(), []byte(port), 0644)
}

func ClearState() {
	_ = os.Remove(pidPath())
	_ = os.Remove(portPath())
}

func Stop() error {
	pid, err := readPID()
	if err != nil {
		return fmt.Errorf("agent server is not running")
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		ClearState()
		return nil
	}
	if runtime.GOOS == "windows" {
		err = proc.Kill()
	} else {
		err = proc.Signal(syscall.SIGTERM)
	}
	if err != nil {
		ClearState()
		return fmt.Errorf("stop process %d: %w", pid, err)
	}

	for i := 0; i < 50; i++ {
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	ClearState()
	return nil
}

func readPID() (int, error) {
	data, err := os.ReadFile(pidPath())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(data))
}

func StartBackground(args []string) (int, error) {
	if os.Getenv("TAC_AGENT_DAEMON") == "1" {
		return os.Getppid(), nil
	}
	if IsRunning() {
		port, _ := ReadPort()
		return 0, fmt.Errorf("agent server already running on %s", port)
	}

	ClearState()

	exe, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("get executable: %w", err)
	}

	env := os.Environ()
	env = append(env, "TAC_AGENT_DAEMON=1")

	pid, err := startBackgroundProc(exe, args, env)
	if err != nil {
		return 0, fmt.Errorf("start background process: %w", err)
	}
	return pid, nil
}
