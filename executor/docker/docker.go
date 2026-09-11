package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/LerkoX/flowx/executor"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"gopkg.in/yaml.v2"
)

// DockerExecutor Docker执行器实现
type DockerExecutor struct {
	client            *client.Client
	containerID       string
	image             string
	workdir           string
	env               map[string]string
	volumes           map[string]string
	network           string
	registry          string
	host              string // daemon 地址（tcp://… / ssh://… / unix://…），空表示从环境变量读取（DOCKER_HOST 等）
	tlsVerify         bool   // 是否启用 TLS 校验
	certPath          string // TLS 证书目录（含 ca.pem/cert.pem/key.pem），默认为 ~/.docker
	tty               bool   // 是否启用 TTY 模式
	ttyHeight         uint   // TTY 终端高度
	ttyWidth          uint   // TTY 终端宽度
	currentExecCancel context.CancelFunc // 用于取消当前执行的命令
	mu                sync.RWMutex
}

// parseInputRequest 解析输入请求代码块内容
// 支持 YAML 或 JSON 格式
func parseInputRequest(content string) *executor.InputRequest {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	var req executor.InputRequest

	if err := yaml.Unmarshal([]byte(content), &req); err == nil && req.Type != "" {
		return &req
	}

	if err := json.Unmarshal([]byte(content), &req); err == nil && req.Type != "" {
		return &req
	}

	return nil
}

// NewDockerExecutor 创建新的Docker执行器
//
// client 不在此处创建，而是在 Prepare 时惰性创建（ensureClient）：
// 此时 adapter 配置（host/tlsVerify/certPath 等）已应用完毕，
// 才能决定连接哪个 daemon。未配置 host 时回退到环境变量（FromEnv），
// 与历史行为一致。
func NewDockerExecutor() (*DockerExecutor, error) {
	return &DockerExecutor{
		env:     make(map[string]string),
		volumes: make(map[string]string),
	}, nil
}

// ensureClient 惰性创建 Docker client（调用方须持有 d.mu）。
// 配置了 host 时按 host/tlsVerify/certPath 构造；否则读取进程环境变量
// （DOCKER_HOST / DOCKER_TLS_VERIFY / DOCKER_CERT_PATH / DOCKER_API_VERSION）。
func (d *DockerExecutor) ensureClient() error {
	if d.client != nil {
		return nil
	}

	opts := []client.Opt{client.WithAPIVersionNegotiation()}
	if d.host != "" {
		opts = append(opts, client.WithHost(d.host))
		if d.tlsVerify || d.certPath != "" {
			certDir := d.certPath
			if certDir == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("tlsVerify requires certPath (failed to locate home dir): %w", err)
				}
				certDir = filepath.Join(home, ".docker")
			}
			opts = append(opts, client.WithTLSClientConfig(
				filepath.Join(certDir, "ca.pem"),
				filepath.Join(certDir, "cert.pem"),
				filepath.Join(certDir, "key.pem"),
			))
		}
	} else {
		opts = append([]client.Opt{client.FromEnv}, opts...)
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return fmt.Errorf("failed to create docker client (host=%q): %w", d.host, err)
	}
	d.client = cli
	return nil
}

// NewDockerExecutorWithClient 使用指定的Docker客户端创建执行器
func NewDockerExecutorWithClient(cli *client.Client) *DockerExecutor {
	return &DockerExecutor{
		client:  cli,
		env:     make(map[string]string),
		volumes: make(map[string]string),
	}
}

// Prepare 准备Docker环境
// 1. 检查/拉取镜像
// 2. 创建并启动容器
func (d *DockerExecutor) Prepare(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 惰性创建 client（此时 host/tlsVerify/certPath 等配置已应用）
	if err := d.ensureClient(); err != nil {
		return err
	}

	// 如果没有指定镜像，使用默认镜像
	if d.image == "" {
		d.image = "alpine:latest"
	}

	// 解析镜像名称（处理registry）
	fullImage := d.resolveImageName()

	// 检查镜像是否存在，不存在则拉取
	if err := d.pullImageIfNeeded(ctx, fullImage); err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	// 构建容器配置
	// 当启用 TTY 时，容器本身也需要开启 TTY，以保证 exec attach 的 TTY 模式能正常工作
	containerConfig := &container.Config{
		Image:        fullImage,
		Cmd:          []string{"sleep", "3600"},
		WorkingDir:   d.workdir,
		Env:          d.buildEnvList(),
		AttachStdout: true,
		AttachStderr: true,
		Tty:          d.tty,
		OpenStdin:    d.tty,
	}

	// 构建主机配置
	hostConfig := &container.HostConfig{
		Mounts:     d.buildMounts(),
		AutoRemove: false,
	}

	// 设置网络模式
	if d.network != "" {
		hostConfig.NetworkMode = container.NetworkMode(d.network)
	}

	// 创建容器
	resp, err := d.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, fmt.Sprintf("flowx-%d", time.Now().UnixNano()))
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	d.containerID = resp.ID

	// 启动容器
	if err := d.client.ContainerStart(ctx, d.containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// 等待容器启动完成
	if err := d.waitForContainer(ctx); err != nil {
		return fmt.Errorf("container failed to start: %w", err)
	}

	return nil
}

// Destruction 销毁Docker环境
// 停止并删除容器
func (d *DockerExecutor) Destruction(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.containerID == "" {
		return nil
	}

	// 停止容器
	timeout := 10
	_ = d.client.ContainerStop(ctx, d.containerID, container.StopOptions{
		Timeout: &timeout,
	})

	// 删除容器
	if err := d.client.ContainerRemove(ctx, d.containerID, container.RemoveOptions{
		Force: true,
	}); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	d.containerID = ""
	return nil
}

// Transfer 在Docker容器中执行命令
// in 接收执行数据（包括步骤信息），out 发送执行结果
// inputChan 用于接收交互式输入数据，可为 nil（不需要输入时）
//
// 当 ctx 被取消时，会立即停止执行新命令，并终止当前正在容器内执行的命令
func (d *DockerExecutor) Transfer(ctx context.Context, resultChan chan<- any, commandChan <-chan any, inputChan <-chan []byte) {
	// 创建一个可取消的内部上下文，用于控制当前命令的执行
	execCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 监听 commandChan 关闭，确保监听 goroutine 能正确退出
	commandChanDone := make(chan struct{})
	go func() {
		for range commandChan {
		}
		close(commandChanDone)
	}()

	// 启动一个 goroutine 监听外部上下文取消和 commandChan 关闭
	go func() {
		select {
		case <-ctx.Done():
		case <-commandChanDone:
		}
		cancel()
	}()

	for {
		// 检查上下文是否已取消
		select {
		case <-execCtx.Done():
			return
		case data, ok := <-commandChan:
			if !ok {
				return
			}

			// 处理 commandWrapper 类型
			cmdWrapper, ok := data.(executor.CommandWrapper)
			if !ok {
				safeSend(resultChan, fmt.Errorf("unsupported data type: %T, expected CommandWrapper", data))
				continue
			}
			// 执行命令（携带步骤名称）
			d.executeCommandStreaming(execCtx, cmdWrapper.Command, cmdWrapper.StepName, cmdWrapper.Env, resultChan, inputChan)
		}
	}
}

// executeCommandStreaming 执行命令并实时流式输出
func (d *DockerExecutor) executeCommandStreaming(ctx context.Context, command string, stepName string, env map[string]string, resultChan chan<- any, inputChan <-chan []byte) {
	startTime := time.Now()

	inputRequestChan := make(chan *executor.InputRequest, 1)
	onInputRequest := func(req *executor.InputRequest) {
		select {
		case inputRequestChan <- req:
		default:
		}
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
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

	err := d.executeCommandInContainerStreaming(ctx, command, env, func(data []byte) {
		safeSend(resultChan, data)
	}, inputChan, onInputRequest)

	// 发送最终结果
	safeSend(resultChan, &executor.StepResult{
		StepName:   stepName,
		Command:    command,
		Output:     "",
		Error:      err,
		StartTime:  startTime,
		FinishTime: time.Now(),
	})
}

// executeCommandInContainerStreaming 在容器中执行命令并实时流式输出
func (d *DockerExecutor) executeCommandInContainerStreaming(ctx context.Context, command string, env map[string]string, outputCallback func([]byte), inputChan <-chan []byte, onInputRequest func(*executor.InputRequest)) error {
	d.mu.RLock()
	containerID := d.containerID
	d.mu.RUnlock()

	if containerID == "" {
		return fmt.Errorf("container not prepared")
	}

	shell := d.detectShell()

	execConfig := container.ExecOptions{
		Cmd:          []string{shell, "-c", command},
		AttachStdout: true,
		AttachStderr: true,
		AttachStdin:  inputChan != nil,
		Tty:          d.tty,
	}
	// 命令级环境变量（dag 渲染终值）经 exec 配置真实注入，不经 shell 解析
	if len(env) > 0 {
		envList := make([]string, 0, len(env))
		for k, v := range env {
			envList = append(envList, k+"="+v)
		}
		execConfig.Env = envList
	}

	execResp, err := d.client.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return fmt.Errorf("failed to create exec: %w", err)
	}

	attachResp, err := d.client.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{
		Tty: d.tty,
	})
	if err != nil {
		return fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer attachResp.Close()

	// 如果启用 TTY，应用终端尺寸
	if d.tty && (d.ttyWidth > 0 || d.ttyHeight > 0) {
		_ = d.client.ContainerExecResize(ctx, execResp.ID, container.ResizeOptions{
			Width:  d.ttyWidth,
			Height: d.ttyHeight,
		})
	}

	var wg sync.WaitGroup
	done := make(chan struct{})

	execCtx, execCancel := context.WithCancel(ctx)
	defer execCancel()

	// 使用 sync.Once 保证取消逻辑只执行一次，避免重复关闭连接或重复取消
	var cancelOnce sync.Once
	d.mu.Lock()
	d.currentExecCancel = func() {
		cancelOnce.Do(func() {
			if attachResp.Conn != nil {
				_, _ = attachResp.Conn.Write([]byte{0x03})
			}
			execCancel()
		})
	}
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		d.currentExecCancel = nil
		d.mu.Unlock()
	}()

	go func() {
		<-ctx.Done()
		d.mu.RLock()
		cancel := d.currentExecCancel
		d.mu.RUnlock()
		if cancel != nil {
			cancel()
		}
	}()

	if inputChan != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-execCtx.Done():
					return
				case <-done:
					return
				case data, ok := <-inputChan:
					if !ok {
						return
					}
					if len(data) > 0 && attachResp.Conn != nil {
						_, _ = attachResp.Conn.Write(data)
					}
				}
			}
		}()
	}

	scanner := bufio.NewScanner(attachResp.Reader)
	scanner.Buffer(make([]byte, 4096), 1024*1024)

	var buffer strings.Builder
	inInputBlock := false

	for scanner.Scan() {
		line := scanner.Text()

		if strings.TrimSpace(line) == "```flowx-input" {
			inInputBlock = true
			buffer.Reset()
			continue
		}

		if inInputBlock && strings.TrimSpace(line) == "```" {
			inInputBlock = false
			if req := parseInputRequest(buffer.String()); req != nil && onInputRequest != nil {
				onInputRequest(req)
			}
			continue
		}

		if inInputBlock {
			buffer.WriteString(line)
			buffer.WriteString("\n")
			continue
		}

		if outputCallback != nil {
			outputCallback(append([]byte(line), '\n'))
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		close(done)
		wg.Wait()
		if outputCallback != nil {
			outputCallback([]byte(fmt.Sprintf("\n[stream error: %v]\n", err)))
		}
		return fmt.Errorf("failed to read output: %w", err)
	}

	close(done)
	wg.Wait()

	for {
		inspectResp, err := d.client.ContainerExecInspect(ctx, execResp.ID)
		if err != nil {
			return fmt.Errorf("failed to inspect exec: %w", err)
		}

		if !inspectResp.Running {
			if inspectResp.ExitCode != 0 {
				return fmt.Errorf("command exited with code %d", inspectResp.ExitCode)
			}
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

// detectShell 检测容器中的shell
func (d *DockerExecutor) detectShell() string {
	// 根据镜像类型选择shell
	image := strings.ToLower(d.image)
	if strings.Contains(image, "alpine") || strings.Contains(image, "busybox") {
		return "/bin/sh"
	}
	return "/bin/bash"
}

// pullImageIfNeeded 检查并拉取镜像
func (d *DockerExecutor) pullImageIfNeeded(ctx context.Context, imageName string) error {
	// 检查镜像是否存在
	_, _, err := d.client.ImageInspectWithRaw(ctx, imageName)
	if err == nil {
		return nil
	}

	// 镜像不存在，需要拉取
	reader, err := d.client.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", imageName, err)
	}
	defer reader.Close()

	// 等待拉取完成（读取所有输出）
	_, _ = io.Copy(io.Discard, reader)

	return nil
}

// waitForContainer 等待容器启动完成
func (d *DockerExecutor) waitForContainer(ctx context.Context) error {
	for i := 0; i < 30; i++ {
		containerJSON, err := d.client.ContainerInspect(ctx, d.containerID)
		if err != nil {
			return err
		}

		if containerJSON.State.Running {
			return nil
		}

		return fmt.Errorf("container exited with code %d", containerJSON.State.ExitCode)
	}

	return fmt.Errorf("timeout waiting for container to start")
}

// resolveImageName 解析完整的镜像名称
func (d *DockerExecutor) resolveImageName() string {
	if d.registry == "" || strings.Contains(d.image, "/") {
		return d.image
	}
	return fmt.Sprintf("%s/%s", d.registry, d.image)
}

// buildEnvList 构建环境变量列表
func (d *DockerExecutor) buildEnvList() []string {
	envList := make([]string, 0, len(d.env))
	for k, v := range d.env {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}
	return envList
}

// buildMounts 构建挂载配置
func (d *DockerExecutor) buildMounts() []mount.Mount {
	mounts := make([]mount.Mount, 0, len(d.volumes))
	for hostPath, containerPath := range d.volumes {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: hostPath,
			Target: containerPath,
		})
	}
	return mounts
}

// setImage 设置镜像
func (d *DockerExecutor) setImage(image string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.image = image
}

// setWorkdir 设置工作目录
func (d *DockerExecutor) setWorkdir(workdir string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.workdir = workdir
}

// setEnv 设置环境变量
func (d *DockerExecutor) setEnv(key, value string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.env[key] = value
}

// setVolume 设置卷挂载
func (d *DockerExecutor) setVolume(hostPath, containerPath string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.volumes[hostPath] = containerPath
}

// setNetwork 设置网络
func (d *DockerExecutor) setNetwork(network string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.network = network
}

// setRegistry 设置镜像仓库
func (d *DockerExecutor) setRegistry(registry string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.registry = registry
}

// setHost 设置 Docker daemon 地址（如 tcp://192.168.1.10:2375、ssh://user@host）。
// 空字符串表示从环境变量读取（DOCKER_HOST 等）。
func (d *DockerExecutor) setHost(host string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.host = host
}

// setTLSVerify 设置是否对 daemon 连接启用 TLS 校验
func (d *DockerExecutor) setTLSVerify(verify bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.tlsVerify = verify
}

// setCertPath 设置 TLS 证书目录（目录内需含 ca.pem / cert.pem / key.pem）
func (d *DockerExecutor) setCertPath(certPath string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.certPath = certPath
}

// setTTY 设置是否启用 TTY 模式
func (d *DockerExecutor) setTTY(enabled bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.tty = enabled
}

// setTTYSize 设置 TTY 终端尺寸
func (d *DockerExecutor) setTTYSize(width, height uint) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ttyWidth = width
	d.ttyHeight = height
}

// GetContainerID 获取容器ID
func (d *DockerExecutor) GetContainerID() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.containerID
}

// GetRuntimeInfo 获取运行时信息
func (d *DockerExecutor) GetRuntimeInfo() map[string]any {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return map[string]any{
		"containerId": d.containerID,
		"image":       d.image,
		"network":     d.network,
		"workdir":     d.workdir,
		"registry":    d.registry,
	}
}

// GetInstanceId 获取实例ID（容器ID）
func (d *DockerExecutor) GetInstanceId() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.containerID
}

// GetType 获取executor类型
func (d *DockerExecutor) GetType() string {
	return "docker"
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

// 确保DockerExecutor实现了Executor接口和ExecutorInfoProvider接口
var _ executor.Executor = (*DockerExecutor)(nil)
var _ executor.ExecutorInfoProvider = (*DockerExecutor)(nil)
