package local

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/LerkoX/flowx/executor"
	"gopkg.in/yaml.v3"
)

// activeCmd 封装当前正在执行的命令及其生命周期信号
type activeCmd struct {
	cmd     *exec.Cmd
	started chan struct{} // 命令已启动（Process 已初始化）
	done    chan struct{} // 命令已完成（Wait 已返回）
	pid     int           // 进程 ID，用于 kill 路径避免与 Wait 竞争
}

// LocalExecutor 本地执行器实现
type LocalExecutor struct {
	workdir    string            // 工作目录
	env        map[string]string // 环境变量
	shell      string            // 使用的shell
	timeout    time.Duration     // 默认超时时间
	usePTY     bool              // 是否使用伪终端（支持交互式命令）
	ptyWidth   int               // 终端宽度
	ptyHeight  int               // 终端高度
	mu         sync.RWMutex
	configMu   sync.RWMutex // 保护配置字段（workdir/env/shell/timeout/usePTY/ptySize）
	cmdMu      sync.Mutex   // 保护当前命令的生命周期
	currentCmd *activeCmd   // 当前执行的命令（用于取消）
}

// NewLocalExecutor 创建新的本地执行器
func NewLocalExecutor() *LocalExecutor {
	return &LocalExecutor{
		env:       make(map[string]string),
		shell:     detectDefaultShell(),
		timeout:   0, // 默认无超时
		usePTY:    false,
		ptyWidth:  80,
		ptyHeight: 24,
	}
}

// Prepare 准备本地执行环境
// 本地执行器不需要特殊的准备，只需要验证工作目录
func (l *LocalExecutor) Prepare(ctx context.Context) error {
	l.configMu.Lock()
	defer l.configMu.Unlock()

	// 如果指定了工作目录，验证它存在
	if l.workdir != "" {
		info, err := os.Stat(l.workdir)
		if err != nil {
			return fmt.Errorf("workdir does not exist: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("workdir is not a directory: %s", l.workdir)
		}
	}

	// 验证shell可用
	if l.shell != "" {
		_, err := exec.LookPath(l.shell)
		if err != nil {
			return fmt.Errorf("shell not found: %s", l.shell)
		}
	}

	return nil
}

// Destruction 销毁本地执行环境
// 本地执行器不需要特殊的清理，但会终止正在运行的命令
func (l *LocalExecutor) Destruction(ctx context.Context) error {
	l.killCurrentProcess()
	return nil
}

// Transfer 接收命令并执行
// 只支持 string 类型的命令
// inputChan 用于接收交互式输入数据，可为 nil（不需要输入时）
//
// 当 ctx 被取消时，会立即停止执行新命令，并终止当前正在执行的进程
func (l *LocalExecutor) Transfer(ctx context.Context, resultChan chan<- any, commandChan <-chan any, inputChan <-chan []byte) {
	// 创建一个可取消的内部上下文，用于控制当前命令的执行
	execCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 启动一个 goroutine 监听外部上下文取消
	// 当外部上下文被取消时，取消内部上下文并终止当前进程
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		select {
		case <-ctx.Done():
			// 外部上下文被取消，取消内部上下文并终止当前进程
			cancel()
			l.killCurrentProcess()
		case <-execCtx.Done():
			// execCtx 被外部正常结束（Transfer 返回）
		}
	}()

	for data := range commandChan {
		// 检查上下文是否已取消
		select {
		case <-execCtx.Done():
			break
		default:
		}

		// 处理 commandWrapper 类型
		cmdWrapper, ok := data.(executor.CommandWrapper)
		if !ok {
			safeSend(resultChan, fmt.Errorf("unsupported data type: %T, expected: CommandWrapper", data))
			continue
		}
		// 执行命令（携带步骤名称）
		l.executeCommandStreaming(execCtx, cmdWrapper.Command, cmdWrapper.StepName, cmdWrapper.Env, resultChan, inputChan)
	}

	// commandChan 关闭后，等待监听 goroutine 退出并关闭 resultChan
	// 给外部接收者一个明确的结束信号
	cancel()
	<-watchDone
	close(resultChan)
}

// killCurrentProcess 终止当前正在执行的进程
// 注意：此函数不调用 cmd.Wait()，避免与 executeCommandWithStreaming 中的 Wait 竞争。
func (l *LocalExecutor) killCurrentProcess() {
	l.cmdMu.Lock()
	ac := l.currentCmd
	l.cmdMu.Unlock()

	if ac == nil {
		return
	}

	// 等待命令启动完成（Process 已初始化）
	select {
	case <-ac.started:
	case <-time.After(5 * time.Second):
		// 命令迟迟没有启动，无法安全终止
		return
	}

	// 使用本地保存的 pid 终止进程（Linux 为整棵进程树，含 shell/script 包裹的孙进程）
	// 先尝试发送中断信号（Unix 进程组 SIGINT）或 Ctrl+Break（Windows）
	if err := interruptProcess(ac.pid); err != nil {
		_ = killProcess(ac.pid)
	} else {
		// 发送信号成功，等待进程退出（最多2秒）
		select {
		case <-ac.done:
			// 进程已退出
		case <-time.After(2 * time.Second):
			// 超时，强制终止
			_ = killProcess(ac.pid)
		}
	}
}

// executeCommandStreaming 执行命令并实时流式输出
func (l *LocalExecutor) executeCommandStreaming(ctx context.Context, command string, stepName string, env map[string]string, resultChan chan<- any, inputChan <-chan []byte) {
	startTime := time.Now()

	// 创建带超时的上下文
	execCtx := ctx
	var timeoutCancel context.CancelFunc
	if l.timeout > 0 {
		execCtx, timeoutCancel = context.WithTimeout(ctx, l.timeout)
		defer timeoutCancel()
	}

	// 输入请求事件通道
	inputRequestChan := make(chan *executor.InputRequest, 1)
	onInputRequest := func(req *executor.InputRequest) {
		select {
		case inputRequestChan <- req:
		default:
		}
	}

	// 启动 goroutine 处理输入请求事件
	inputEventDone := make(chan struct{})
	go func() {
		defer close(inputEventDone)
		for {
			select {
			case <-execCtx.Done():
				return
			case req := <-inputRequestChan:
				if req != nil {
					safeSend(resultChan, &executor.InputRequestEvent{
						StepName: stepName,
						Request:  req,
					})
				}
			}
		}
	}()

	// 使用带超时的上下文执行命令
	err := l.executeCommandWithStreaming(execCtx, command, stepName, env, func(data []byte) {
		safeSend(resultChan, data)
	}, inputChan, onInputRequest)

	// 发送最终结果（必须在等待 inputEventDone 之前，否则 inputEventDone 可能阻塞在发送 InputRequestEvent）
	safeSend(resultChan, &executor.StepResult{
		StepName:   stepName,
		Command:    command,
		Output:     "",
		Error:      err,
		StartTime:  startTime,
		FinishTime: time.Now(),
	})

	// 等待输入事件处理 goroutine 退出
	if timeoutCancel != nil {
		timeoutCancel()
	}
	select {
	case <-inputEventDone:
	case <-time.After(2 * time.Second):
	}
}

// executeCommandWithStreaming 执行命令并实时输出
func (l *LocalExecutor) executeCommandWithStreaming(ctx context.Context, command string, stepName string, env map[string]string, outputCallback func([]byte), inputChan <-chan []byte, onInputRequest func(*executor.InputRequest)) error {
	// 先复制需要的环境变量和配置，避免持有锁期间调用外部函数
	l.configMu.RLock()
	workdir := l.workdir
	envCopy := make(map[string]string, len(l.env)+len(env))
	for k, v := range l.env {
		envCopy[k] = v
	}
	l.configMu.RUnlock()
	// 命令级环境变量（dag 渲染终值）覆盖执行器级同名变量，
	// 以真实进程环境变量注入，不经 shell 解析
	for k, v := range env {
		envCopy[k] = v
	}

	// 创建命令
	cmd := l.createCommand(ctx, command)

	// 设置工作目录
	if workdir != "" {
		cmd.Dir = workdir
	}

	// 设置环境变量
	cmd.Env = buildEnvList(envCopy)

	// 获取stdout和stderr管道
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// 获取stdin管道（如果需要输入）
	var stdin io.WriteCloser
	if inputChan != nil {
		stdin, err = cmd.StdinPipe()
		if err != nil {
			return fmt.Errorf("failed to create stdin pipe: %w", err)
		}
	}

	// 初始化当前命令生命周期对象
	started := make(chan struct{})
	done := make(chan struct{})
	commandDone := make(chan struct{})
	waitOnce := sync.Once{}
	ac := &activeCmd{
		cmd:     cmd,
		started: started,
		done:    done,
	}

	l.cmdMu.Lock()
	l.currentCmd = ac
	l.cmdMu.Unlock()

	// 确保函数退出时清理 currentCmd 并关闭 done
	defer func() {
		l.cmdMu.Lock()
		if l.currentCmd == ac {
			l.currentCmd = nil
		}
		l.cmdMu.Unlock()
		// 确保 done 被关闭，防止 killCurrentProcess 永久等待
		waitOnce.Do(func() {
			close(commandDone)
			close(done)
		})
	}()

	// 启动命令
	if err := cmd.Start(); err != nil {
		close(started)
		return fmt.Errorf("failed to start command: %w", err)
	}

	// 记录 PID 并通知已启动
	ac.pid = cmd.Process.Pid
	close(started)

	// 命令完成信号通道（用于输入 goroutine）
	// 已在上面声明

	// 使用 WaitGroup 等待 stdout/stderr 读取 goroutine 完成
	var outputWg sync.WaitGroup
	outputWg.Add(2)
	go func() {
		defer outputWg.Done()
		l.streamOutput(ctx, stdout, outputCallback, stepName, onInputRequest)
	}()
	go func() {
		defer outputWg.Done()
		l.streamOutput(ctx, stderr, outputCallback, stepName, nil) // stderr 不检测输入请求
	}()

	// 输入处理：从 inputChan 读取并写入 stdin
	var inputWg sync.WaitGroup
	if inputChan != nil {
		inputWg.Add(1)
		go func() {
			defer inputWg.Done()
			for {
				select {
				case <-ctx.Done():
					if stdin != nil {
						_ = stdin.Close()
					}
					return
				case <-commandDone:
					// 命令已完成，关闭 stdin 并退出
					if stdin != nil {
						_ = stdin.Close()
					}
					return
				case data, ok := <-inputChan:
					if !ok {
						if stdin != nil {
							_ = stdin.Close()
						}
						return
					}
					if len(data) > 0 && stdin != nil {
						if _, err := stdin.Write(data); err != nil {
							_ = stdin.Close()
							return
						}
					}
				}
			}
		}()
	}

	// 等待输出读取完成（命令已退出或管道已关闭）
	outputWg.Wait()

	// 关闭 stdout/stderr 读取端，确保 scanner goroutine 退出
	_ = stdout.Close()
	_ = stderr.Close()

	// 等待命令完成（带超时）。ctx 取消时由上层 Transfer.killCurrentProcess 终止进程。
	waitErr := make(chan error, 1)
	go func() {
		defer func() {
			waitOnce.Do(func() {
				close(commandDone)
				close(done)
			})
		}()
		waitErr <- cmd.Wait()
	}()

	select {
	case err = <-waitErr:
		// 命令正常退出
	case <-ctx.Done():
		// 上下文取消，强制终止整棵进程树（使用本地保存的 PID 避免竞争）
		if ac.pid > 0 {
			_ = killProcess(ac.pid)
		}
		// 等待 Wait 返回，避免 goroutine 泄漏
		<-waitErr
		// 使用上下文的错误作为超时错误
		if ctx.Err() == context.DeadlineExceeded {
			err = fmt.Errorf("command timed out: %w", ctx.Err())
		} else {
			err = fmt.Errorf("command cancelled: %w", ctx.Err())
		}
	}

	// 等待输入 goroutine 退出
	inputWg.Wait()

	// 关闭 stdin 读取端以彻底断开输入管道
	if stdin != nil {
		_ = stdin.Close()
	}

	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("command timed out: %w", err)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("command exited with code %d", exitErr.ExitCode())
		}
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}

// streamOutput 读取输出并回调
// 同时检测输入请求代码块 ```flowx-input
func (l *LocalExecutor) streamOutput(ctx context.Context, reader io.Reader, callback func([]byte), stepName string, onInputRequest func(*executor.InputRequest)) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 100*1024*1024) // 增大缓冲区到 100MB，避免单行超大输出导致 token too long

	var buffer strings.Builder
	inInputBlock := false

	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}

		line := scanner.Text()

		// 检测代码块开始
		if strings.TrimSpace(line) == "```flowx-input" {
			inInputBlock = true
			buffer.Reset()
			continue
		}

		// 检测代码块结束
		if inInputBlock && strings.TrimSpace(line) == "```" {
			inInputBlock = false
			// 解析输入请求
			if onInputRequest != nil {
				if req := parseInputRequest(buffer.String()); req != nil {
					onInputRequest(req)
				}
			}
			continue
		}

		// 在代码块内，积累内容
		if inInputBlock {
			buffer.WriteString(line)
			buffer.WriteString("\n")
			continue
		}

		// 普通输出行，传递给回调
		if callback != nil {
			callback(append([]byte(line), '\n'))
		}
	}

	// 扫描错误通常意味着管道已关闭或输出过大，这里不再额外报告
	_ = scanner.Err()
}

// parseInputRequest 解析输入请求代码块内容
// 支持 YAML 或 JSON 格式
func parseInputRequest(content string) *executor.InputRequest {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	var req executor.InputRequest

	// 尝试 YAML 格式
	if err := yaml.Unmarshal([]byte(content), &req); err == nil && req.Type != "" {
		return &req
	}

	// 尝试 JSON 格式
	if err := json.Unmarshal([]byte(content), &req); err == nil && req.Type != "" {
		return &req
	}

	return nil
}

// safeSend 安全地发送数据到 channel，如果 channel 已关闭则忽略
func safeSend(ch chan<- any, value any) {
	defer func() {
		if r := recover(); r != nil {
			// channel 已关闭，忽略
		}
	}()
	ch <- value
}

// createCommand 根据操作系统创建命令
func (l *LocalExecutor) createCommand(ctx context.Context, command string) *exec.Cmd {
	l.configMu.RLock()
	shell := l.shell
	usePTY := l.usePTY
	l.configMu.RUnlock()

	if usePTY {
		return l.createCommandWithPTY(ctx, command)
	}

	switch runtime.GOOS {
	case "windows":
		// Windows使用cmd.exe
		if shell == "powershell" || shell == "pwsh" {
			return prepareCmd(exec.CommandContext(ctx, shell, "-Command", command))
		}
		return prepareCmd(exec.CommandContext(ctx, "cmd", "/C", command))
	default:
		// Unix-like系统使用sh或bash
		if shell == "" {
			shell = "/bin/sh"
		}
		return prepareCmd(exec.CommandContext(ctx, shell, "-c", command))
	}
}

// createCommandWithPTY 创建使用伪终端的命令
func (l *LocalExecutor) createCommandWithPTY(ctx context.Context, command string) *exec.Cmd {
	l.configMu.RLock()
	shell := l.shell
	l.configMu.RUnlock()

	switch runtime.GOOS {
	case "windows":
		// Windows 不支持 PTY，回退到普通命令
		if shell == "powershell" || shell == "pwsh" {
			return prepareCmd(exec.CommandContext(ctx, shell, "-Command", command))
		}
		return prepareCmd(exec.CommandContext(ctx, "cmd", "/C", command))
	default:
		// Unix-like 系统使用 script 命令模拟 PTY
		if shell == "" {
			shell = "/bin/sh"
		}
		// 使用 script 命令创建伪终端；-e 透传子进程退出码（util-linux），
		// 否则节点脚本 exit 非零会被 script 吞掉导致失败节点误报成功
		return prepareCmd(exec.CommandContext(ctx, "script", "-q", "-e", "-c", command, "/dev/null"))
	}
}

// buildEnvList 构建环境变量列表（基于传入的自定义环境变量副本）
func buildEnvList(customEnv map[string]string) []string {
	// 从当前进程环境变量开始
	envMap := make(map[string]string)
	for _, e := range os.Environ() {
		if i := strings.Index(e, "="); i >= 0 {
			envMap[e[:i]] = e[i+1:]
		}
	}

	// 添加自定义环境变量（覆盖现有变量）
	for k, v := range customEnv {
		envMap[k] = v
	}

	// 转换为列表
	envList := make([]string, 0, len(envMap))
	for k, v := range envMap {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}

	return envList
}

// buildEnvList 构建环境变量列表（兼容旧调用，使用 executor 内部环境变量）
func (l *LocalExecutor) buildEnvList() []string {
	l.configMu.RLock()
	defer l.configMu.RUnlock()

	envCopy := make(map[string]string, len(l.env))
	for k, v := range l.env {
		envCopy[k] = v
	}

	return buildEnvList(envCopy)
}

// setWorkdir 设置工作目录
func (l *LocalExecutor) setWorkdir(workdir string) {
	l.configMu.Lock()
	defer l.configMu.Unlock()
	l.workdir = workdir
}

// setEnv 设置环境变量
func (l *LocalExecutor) setEnv(key, value string) {
	l.configMu.Lock()
	defer l.configMu.Unlock()
	l.env[key] = value
}

// setShell 设置shell
func (l *LocalExecutor) setShell(shell string) {
	l.configMu.Lock()
	defer l.configMu.Unlock()
	l.shell = shell
}

// setTimeout 设置默认超时
func (l *LocalExecutor) setTimeout(timeout time.Duration) {
	l.configMu.Lock()
	defer l.configMu.Unlock()
	l.timeout = timeout
}

// setPTY 设置是否使用伪终端
func (l *LocalExecutor) setPTY(enabled bool) {
	l.configMu.Lock()
	defer l.configMu.Unlock()
	l.usePTY = enabled
}

// setPTYSize 设置终端尺寸
func (l *LocalExecutor) setPTYSize(width, height int) {
	l.configMu.Lock()
	defer l.configMu.Unlock()
	l.ptyWidth = width
	l.ptyHeight = height
}

// GetWorkdir 获取工作目录
func (l *LocalExecutor) GetWorkdir() string {
	l.configMu.RLock()
	defer l.configMu.RUnlock()
	return l.workdir
}

// GetShell 获取当前shell
func (l *LocalExecutor) GetShell() string {
	l.configMu.RLock()
	defer l.configMu.RUnlock()
	return l.shell
}

// GetRuntimeInfo 获取运行时信息
func (l *LocalExecutor) GetRuntimeInfo() map[string]any {
	l.configMu.RLock()
	defer l.configMu.RUnlock()
	return map[string]any{
		"workdir": l.workdir,
		"shell":   l.shell,
	}
}

// GetInstanceId 获取实例ID（本地执行器没有持久化的实例，返回空）
func (l *LocalExecutor) GetInstanceId() string {
	// Local executor 没有持久化的实例，返回空
	return ""
}

// GetType 获取executor类型
func (l *LocalExecutor) GetType() string {
	return "local"
}

// detectDefaultShell 检测系统默认shell
func detectDefaultShell() string {
	switch runtime.GOOS {
	case "windows":
		// Windows优先使用PowerShell，回退到cmd
		if path, err := exec.LookPath("pwsh"); err == nil {
			return path
		}
		if path, err := exec.LookPath("powershell"); err == nil {
			return path
		}
		return "cmd"
	default:
		// Unix-like系统优先使用bash，回退到sh，均通过PATH查找真实路径
		if path, err := exec.LookPath("bash"); err == nil {
			return path
		}
		if path, err := exec.LookPath("sh"); err == nil {
			return path
		}
		return "/bin/sh"
	}
}

// 确保LocalExecutor实现了Executor接口和ExecutorInfoProvider接口
var _ executor.Executor = (*LocalExecutor)(nil)
var _ executor.ExecutorInfoProvider = (*LocalExecutor)(nil)
