package runtime

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	managerconfig "github.com/openwrt-singbox/singbox-manager/internal/config"
	"github.com/openwrt-singbox/singbox-manager/internal/firewall"
)

const (
	ActionStart Action = iota
	ActionStop
	ActionRestart
	ActionReload

	maxTProxyPolicyRuleDeletes = 64
	runtimeMonitorInterval     = 5 * time.Second
	runtimeRestartBackoff      = 5 * time.Second
)

type Action int

type Paths struct {
	GeneratedConfig string
	RuntimeConfig   string
	PIDFile         string
	NftablesInclude string
}

type Result struct {
	GeneratedPath string
	RuntimePath   string
	NftablesPath  string
	CheckOutput   string
	Message       string
	PID           int
}

type Renderer func(managerconfig.Config) ([]byte, error)

var DefaultPaths = Paths{
	GeneratedConfig: "/etc/singbox-manager/generated/config.json",
	RuntimeConfig:   "/var/run/sing-box/config.json",
	PIDFile:         "/var/run/sing-box/sing-box.pid",
	NftablesInclude: firewall.DefaultIncludePath,
}

var FirewallReload = reloadFirewall
var routeCommand = runRouteCommand

var supervisor runtimeSupervisor

type runtimeSupervisor struct {
	mu             sync.Mutex
	desired        bool
	monitorRunning bool
	cfg            managerconfig.Config
	paths          Paths
	renderer       Renderer
}

func Validate(cfg managerconfig.Config, paths Paths, renderer Renderer) (Result, error) {
	result, err := Generate(cfg, paths, renderer)
	if err != nil {
		return result, err
	}
	output, err := runSingBoxCheck(cfg.Manager.SingBoxBinary, paths.RuntimeConfig)
	result.CheckOutput = output
	if err != nil {
		return result, err
	}
	result.Message = "configuration is valid"
	return result, nil
}

func Control(cfg managerconfig.Config, action Action, paths Paths, renderer Renderer) (Result, error) {
	switch action {
	case ActionStart:
		return start(cfg, paths, renderer)
	case ActionStop:
		return stop(cfg, paths)
	case ActionRestart:
		result, err := stop(cfg, paths)
		if err != nil {
			return result, err
		}
		return start(cfg, paths, renderer)
	case ActionReload:
		return reload(cfg, paths, renderer)
	default:
		return Result{}, fmt.Errorf("unsupported runtime action")
	}
}

func Supervise(cfg managerconfig.Config, paths Paths, renderer Renderer) {
	enableSupervisor(cfg, paths, renderer)
}

func Generate(cfg managerconfig.Config, paths Paths, renderer Renderer) (Result, error) {
	if renderer == nil {
		return Result{}, fmt.Errorf("renderer is required")
	}

	data, err := renderer(cfg)
	if err != nil {
		return Result{}, err
	}

	if err := writeFileAtomic(paths.GeneratedConfig, data, 0644); err != nil {
		return Result{}, err
	}
	if err := writeFileAtomic(paths.RuntimeConfig, data, 0644); err != nil {
		return Result{}, err
	}

	return Result{
		GeneratedPath: paths.GeneratedConfig,
		RuntimePath:   paths.RuntimeConfig,
		NftablesPath:  paths.NftablesInclude,
	}, nil
}

func start(cfg managerconfig.Config, paths Paths, renderer Renderer) (Result, error) {
	result, err := startOnce(cfg, paths, renderer)
	if err != nil {
		return result, err
	}
	enableSupervisor(cfg, paths, renderer)
	return result, nil
}

func startOnce(cfg managerconfig.Config, paths Paths, renderer Renderer) (Result, error) {
	if !cfg.Manager.Enabled {
		return Result{Message: "manager is disabled"}, fmt.Errorf("manager is disabled")
	}

	if pid := RunningPID(paths); pid > 0 {
		return Result{
			GeneratedPath: paths.GeneratedConfig,
			RuntimePath:   paths.RuntimeConfig,
			Message:       "sing-box is already running",
			PID:           pid,
		}, nil
	}

	result, err := Validate(cfg, paths, renderer)
	if err != nil {
		return result, err
	}
	if err := applyFirewall(cfg, paths, &result); err != nil {
		if cleanupErr := cleanupFirewall(cfg, paths, &result); cleanupErr != nil {
			return result, errors.Join(err, fmt.Errorf("cleanup after firewall failure: %w", cleanupErr))
		}
		return result, err
	}

	cmd := exec.Command(cfg.Manager.SingBoxBinary, "run", "-c", paths.RuntimeConfig)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return result, cleanupAfterStartFailure(cfg, paths, &result, err)
	}
	if err := writeFileAtomic(paths.PIDFile, []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0644); err != nil {
		_ = cmd.Process.Kill()
		return result, cleanupAfterStartFailure(cfg, paths, &result, err)
	}
	exited := make(chan error, 1)
	go func(pid int) {
		err := cmd.Wait()
		removePIDFileIfMatches(paths.PIDFile, pid)
		exited <- err
	}(cmd.Process.Pid)
	select {
	case err := <-exited:
		if err != nil {
			return result, cleanupAfterStartFailure(cfg, paths, &result, fmt.Errorf("sing-box exited immediately: %w", err))
		}
		return result, cleanupAfterStartFailure(cfg, paths, &result, fmt.Errorf("sing-box exited immediately"))
	case <-time.After(250 * time.Millisecond):
	}

	result.PID = cmd.Process.Pid
	result.Message = "sing-box started"
	return result, nil
}

func stop(cfg managerconfig.Config, paths Paths) (Result, error) {
	disableSupervisor()
	result := Result{
		GeneratedPath: paths.GeneratedConfig,
		RuntimePath:   paths.RuntimeConfig,
		NftablesPath:  paths.NftablesInclude,
	}
	pid := RunningPID(paths)
	if pid == 0 {
		_ = os.Remove(paths.PIDFile)
		if err := cleanupFirewall(cfg, paths, &result); err != nil {
			return result, err
		}
		result.Message = "sing-box is not running"
		return result, nil
	}

	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
		return result, err
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !pidAlive(pid) {
			_ = os.Remove(paths.PIDFile)
			if err := cleanupFirewall(cfg, paths, &result); err != nil {
				return result, err
			}
			result.Message = "sing-box stopped"
			return result, nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		return result, err
	}
	_ = os.Remove(paths.PIDFile)
	if err := cleanupFirewall(cfg, paths, &result); err != nil {
		return result, err
	}
	result.Message = "sing-box stopped"
	return result, nil
}

func reload(cfg managerconfig.Config, paths Paths, renderer Renderer) (Result, error) {
	result, err := Validate(cfg, paths, renderer)
	if err != nil {
		return result, err
	}

	pid := RunningPID(paths)
	if pid == 0 {
		started, err := start(cfg, paths, renderer)
		if err != nil {
			return started, err
		}
		started.Message = "sing-box started after reload request"
		return started, nil
	}

	if err := applyFirewall(cfg, paths, &result); err != nil {
		return result, err
	}
	if err := syscall.Kill(pid, syscall.SIGHUP); err != nil {
		return result, err
	}
	enableSupervisor(cfg, paths, renderer)
	result.PID = pid
	result.Message = "sing-box reloaded"
	return result, nil
}

func applyFirewall(cfg managerconfig.Config, paths Paths, result *Result) error {
	if err := firewall.Apply(cfg, paths.NftablesInclude); err != nil {
		return err
	}
	if cfg.TProxy.Enabled {
		if err := applyTProxyRoutes(); err != nil {
			return err
		}
		result.NftablesPath = paths.NftablesInclude
		if FirewallReload != nil {
			return FirewallReload()
		}
		return nil
	}
	return reloadFirewallAfterCleanup()
}

func cleanupFirewall(cfg managerconfig.Config, paths Paths, result *Result) error {
	if cfg.TProxy.Enabled && cfg.TProxy.KillSwitch {
		return applyKillSwitchFirewall(cfg, paths, result)
	}
	if err := firewall.Cleanup(paths.NftablesInclude); err != nil {
		return err
	}
	if err := cleanupTProxyRoutes(); err != nil {
		return err
	}
	result.NftablesPath = paths.NftablesInclude
	return reloadFirewallAfterCleanup()
}

func applyKillSwitchFirewall(cfg managerconfig.Config, paths Paths, result *Result) error {
	if err := firewall.ApplyKillSwitch(cfg, paths.NftablesInclude); err != nil {
		return err
	}
	if err := cleanupTProxyRoutes(); err != nil {
		return err
	}
	result.NftablesPath = paths.NftablesInclude
	return reloadFirewallAfterCleanup()
}

func cleanupAfterStartFailure(cfg managerconfig.Config, paths Paths, result *Result, cause error) error {
	if err := cleanupFirewall(cfg, paths, result); err != nil {
		return errors.Join(cause, fmt.Errorf("cleanup after start failure: %w", err))
	}
	return cause
}

func reloadFirewallAfterCleanup() error {
	if FirewallReload == nil {
		return nil
	}
	return FirewallReload()
}

func RunningPID(paths Paths) int {
	data, err := os.ReadFile(paths.PIDFile)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0
	}
	if !pidAlive(pid) {
		_ = os.Remove(paths.PIDFile)
		return 0
	}
	return pid
}

func runSingBoxCheck(binary string, configPath string) (string, error) {
	out, err := exec.Command(binary, "check", "-c", configPath).CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		if output == "" {
			return output, err
		}
		return output, fmt.Errorf("%w: %s", err, output)
	}
	return output, nil
}

func reloadFirewall() error {
	if _, err := os.Stat("/etc/init.d/firewall"); os.IsNotExist(err) {
		return nil
	}
	out, err := exec.Command("/etc/init.d/firewall", "reload").CombinedOutput()
	if err != nil {
		output := strings.TrimSpace(string(out))
		if output == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, output)
	}
	return nil
}

func applyTProxyRoutes() error {
	commands := [][]string{
		{"ip", "-4", "route", "add", "local", "0.0.0.0/0", "dev", "lo", "table", "100"},
		{"ip", "-6", "route", "add", "local", "::/0", "dev", "lo", "table", "100"},
	}
	for _, command := range commands {
		if err := routeCommand(command...); err != nil && !isRouteExistsError(err) {
			return err
		}
	}
	for _, family := range []string{"-4", "-6"} {
		if err := deleteTProxyPolicyRules(family); err != nil {
			return err
		}
		command := []string{"ip", family, "rule", "add", "fwmark", "0x1", "lookup", "100"}
		if err := routeCommand(command...); err != nil && !isRouteExistsError(err) {
			return err
		}
	}
	return nil
}

func cleanupTProxyRoutes() error {
	for _, family := range []string{"-4", "-6"} {
		if err := deleteTProxyPolicyRules(family); err != nil {
			return err
		}
	}
	commands := [][]string{
		{"ip", "-4", "route", "del", "local", "0.0.0.0/0", "dev", "lo", "table", "100"},
		{"ip", "-6", "route", "del", "local", "::/0", "dev", "lo", "table", "100"},
	}
	for _, command := range commands {
		if err := routeCommand(command...); err != nil && !isRouteMissingError(err) {
			return err
		}
	}
	return nil
}

func enableSupervisor(cfg managerconfig.Config, paths Paths, renderer Renderer) {
	supervisor.mu.Lock()
	supervisor.desired = true
	supervisor.cfg = cfg
	supervisor.paths = paths
	supervisor.renderer = renderer
	if supervisor.monitorRunning {
		supervisor.mu.Unlock()
		return
	}
	supervisor.monitorRunning = true
	supervisor.mu.Unlock()

	go supervisorLoop()
}

func disableSupervisor() {
	supervisor.mu.Lock()
	supervisor.desired = false
	supervisor.mu.Unlock()
}

func supervisorSnapshot() (bool, managerconfig.Config, Paths, Renderer) {
	supervisor.mu.Lock()
	defer supervisor.mu.Unlock()
	return supervisor.desired, supervisor.cfg, supervisor.paths, supervisor.renderer
}

func supervisorStopped() {
	supervisor.mu.Lock()
	if supervisor.desired {
		supervisor.mu.Unlock()
		go supervisorLoop()
		return
	}
	supervisor.monitorRunning = false
	supervisor.mu.Unlock()
}

func supervisorLoop() {
	defer supervisorStopped()
	for {
		time.Sleep(runtimeMonitorInterval)
		desired, cfg, paths, renderer := supervisorSnapshot()
		if !desired {
			return
		}
		if RunningPID(paths) > 0 {
			continue
		}
		log.Printf("sing-box runtime is not running; restarting")
		result, err := startOnce(cfg, paths, renderer)
		if err != nil {
			log.Printf("sing-box runtime restart failed: %v", err)
			time.Sleep(runtimeRestartBackoff)
			continue
		}
		if result.PID > 0 {
			log.Printf("sing-box runtime restarted with pid %d", result.PID)
		}
	}
}

func deleteTProxyPolicyRules(family string) error {
	command := []string{"ip", family, "rule", "del", "fwmark", "0x1", "lookup", "100"}
	for i := 0; i < maxTProxyPolicyRuleDeletes; i++ {
		err := routeCommand(command...)
		if err == nil {
			continue
		}
		if isRouteMissingError(err) {
			return nil
		}
		return err
	}
	return fmt.Errorf("too many duplicate tproxy policy rules for %s", family)
}

func runRouteCommand(args ...string) error {
	if len(args) == 0 {
		return nil
	}
	out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	if err != nil {
		output := strings.TrimSpace(string(out))
		if output == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, output)
	}
	return nil
}

func isRouteExistsError(err error) bool {
	return strings.Contains(err.Error(), "File exists")
}

func isRouteMissingError(err error) bool {
	text := err.Error()
	return strings.Contains(text, "No such process") ||
		strings.Contains(text, "No such file or directory") ||
		strings.Contains(text, "Cannot find")
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func pidAlive(pid int) bool {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err == nil {
		if alive, ok := statusAlive(data); ok {
			return alive
		}
	}
	return syscall.Kill(pid, 0) == nil
}

func statusAlive(data []byte) (bool, bool) {
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "State:") {
			return !strings.Contains(line, "Z"), true
		}
	}
	return false, false
}

func removePIDFileIfMatches(path string, pid int) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	current, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err == nil && current == pid {
		_ = os.Remove(path)
	}
}
