package pipelinex

import (
	"gopkg.in/yaml.v2"
)

// Snapshotter 状态快照接口
type Snapshotter interface {
	// TakeSnapshot 从 Pipeline 生成带状态的配置
	TakeSnapshot(pipeline Pipeline, originalConfig *PipelineConfig) (*PipelineConfig, error)
	// ToYAML 将配置转换为 YAML
	ToYAML(config *PipelineConfig) (string, error)
	// FromYAML 从 YAML 解析配置
	FromYAML(yamlStr string) (*PipelineConfig, error)
}

// PipelineSnapshotter 实现
type PipelineSnapshotter struct{}

func NewPipelineSnapshotter() *PipelineSnapshotter {
	return &PipelineSnapshotter{}
}

// TakeSnapshot 将当前状态序列化为 PipelineConfig
func (ps *PipelineSnapshotter) TakeSnapshot(pipeline Pipeline, originalConfig *PipelineConfig) (*PipelineConfig, error) {
	// 深拷贝原始配置
	configBytes, err := yaml.Marshal(originalConfig)
	if err != nil {
		return nil, err
	}

	var config PipelineConfig
	if err := yaml.Unmarshal(configBytes, &config); err != nil {
		return nil, err
	}

	// 更新 Nodes 的 runtime 状态
	graph := pipeline.GetGraph()
	for nodeName, node := range graph.Nodes() {
		if nodeConfig, exists := config.Nodes[nodeName]; exists {
			// 更新节点ID和运行时状态
			if rt := node.GetRuntimeStatus(); rt != nil {
				nodeConfig.Id = rt.Id
				// 创建 runtime 的深拷贝
				runtimeCopy := *rt
				// 深拷贝 Steps
				if len(rt.Steps) > 0 {
					runtimeCopy.Steps = make([]StepRuntimeStatus, len(rt.Steps))
					copy(runtimeCopy.Steps, rt.Steps)
				}
				// 深拷贝 Executor 信息
				if rt.Executor != nil {
					execCopy := *rt.Executor
					if len(rt.Executor.Info) > 0 {
						execCopy.Info = make(map[string]interface{})
						for k, v := range rt.Executor.Info {
							execCopy.Info[k] = v
						}
					}
					runtimeCopy.Executor = &execCopy
				}
				// 深拷贝 Custom 字段
				if len(rt.Custom) > 0 {
					runtimeCopy.Custom = make(map[string]interface{})
					for k, v := range rt.Custom {
						runtimeCopy.Custom[k] = v
					}
				}
				nodeConfig.Runtime = &runtimeCopy
			}
			// 更新步骤ID
			nodeSteps := node.GetSteps()
			for i := range nodeConfig.Steps {
				for _, step := range nodeSteps {
					if nodeConfig.Steps[i].Name == step.Name {
						nodeConfig.Steps[i].Id = step.Id
						break
					}
				}
			}
			config.Nodes[nodeName] = nodeConfig
		}
	}

	return &config, nil
}

// ToYAML 将配置转换为 YAML
func (ps *PipelineSnapshotter) ToYAML(config *PipelineConfig) (string, error) {
	data, err := yaml.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromYAML 从 YAML 解析配置
func (ps *PipelineSnapshotter) FromYAML(yamlStr string) (*PipelineConfig, error) {
	var config PipelineConfig
	err := yaml.Unmarshal([]byte(yamlStr), &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}
