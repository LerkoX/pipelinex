package pipelinex

import (
	"github.com/spf13/cast"
	"github.com/thoas/go-funk"
)

type DGANode struct {
	id            string
	state         string
	property      map[string]any
	pipelineId    string
	executor      string
	steps         []Step
	image         string
	config        map[string]any
	runtimeStatus *NodeRuntimeStatus
}

// NewDGANode creates a new DGANode with the specified id and state, initializing an empty property map.
func NewDGANode(id, state string) *DGANode {
	return &DGANode{
		id:       id,
		state:    state,
		property: map[string]any{},
		steps:    []Step{},
		config:   map[string]any{},
	}
}

// NewDGANodeWithConfig creates a new DGANode with full configuration.
func NewDGANodeWithConfig(id, state, executor, image string, steps []Step, config map[string]any) *DGANode {
	return &DGANode{
		id:       id,
		state:    state,
		property: map[string]any{},
		executor: executor,
		steps:    steps,
		image:    image,
		config:   config,
	}
}

func (dgaNode *DGANode) Id() string {
	return dgaNode.id
}

func (dgaNode *DGANode) PipelineId() string {
	return dgaNode.pipelineId
}

func (dgaNode *DGANode) Status() string {
	return dgaNode.state
}

func (dgaNode *DGANode) Get(key string) string {
	return cast.ToString(funk.Get(dgaNode.property, key))
}

func (dgaNode *DGANode) Set(key string, value any) {
	dgaNode.property[key] = value
}

func (dgaNode *DGANode) GetExecutor() string {
	return dgaNode.executor
}

func (dgaNode *DGANode) GetSteps() []Step {
	return dgaNode.steps
}

func (dgaNode *DGANode) GetImage() string {
	return dgaNode.image
}

func (dgaNode *DGANode) GetConfig() map[string]any {
	return dgaNode.config
}

// GetRuntimeStatus 获取运行时状态
func (dgaNode *DGANode) GetRuntimeStatus() *NodeRuntimeStatus {
	return dgaNode.runtimeStatus
}

// SetRuntimeStatus 设置运行时状态
func (dgaNode *DGANode) SetRuntimeStatus(status *NodeRuntimeStatus) {
	dgaNode.runtimeStatus = status
}

// GetStepRuntimeStatus 获取指定步骤的运行时状态
func (dgaNode *DGANode) GetStepRuntimeStatus(stepName string) *StepRuntimeStatus {
	if dgaNode.runtimeStatus == nil {
		return nil
	}
	for i := range dgaNode.runtimeStatus.Steps {
		if dgaNode.runtimeStatus.Steps[i].Name == stepName {
			return &dgaNode.runtimeStatus.Steps[i]
		}
	}
	return nil
}

// SetStepRuntimeStatus 设置步骤运行时状态
func (dgaNode *DGANode) SetStepRuntimeStatus(stepStatus *StepRuntimeStatus) {
	if dgaNode.runtimeStatus == nil {
		dgaNode.runtimeStatus = &NodeRuntimeStatus{
			Id:     NewUUID(),
			Status: StatusUnknown,
			Steps:  []StepRuntimeStatus{},
		}
	}
	// 查找并更新或添加
	found := false
	for i := range dgaNode.runtimeStatus.Steps {
		if dgaNode.runtimeStatus.Steps[i].Name == stepStatus.Name {
			dgaNode.runtimeStatus.Steps[i] = *stepStatus
			found = true
			break
		}
	}
	if !found {
		dgaNode.runtimeStatus.Steps = append(dgaNode.runtimeStatus.Steps, *stepStatus)
	}
}

// EnsureIds 确保节点和步骤都有ID
func (dgaNode *DGANode) EnsureIds() {
	if dgaNode.runtimeStatus == nil {
		dgaNode.runtimeStatus = &NodeRuntimeStatus{
			Id:     NewUUID(),
			Status: StatusUnknown,
			Steps:  []StepRuntimeStatus{},
		}
	} else if dgaNode.runtimeStatus.Id == "" {
		dgaNode.runtimeStatus.Id = NewUUID()
	}
	// 确保步骤都有ID
	for i := range dgaNode.steps {
		if dgaNode.steps[i].Id == "" {
			dgaNode.steps[i].Id = NewUUID()
		}
	}
}
