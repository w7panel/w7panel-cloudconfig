package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	CloudConfigKind          = "CloudConfig"
	CloudConfigResource      = "cloudconfigs"
	CloudConfigLabelManaged  = "cloudconfig.w7.cc/managed"
	CloudConfigLabelName     = "cloudconfig.w7.cc/name"
	DeployConfigMapLabel     = "cloudconfig.w7.cc/deploy"
	DeployConfigStrategyID   = "cloudconfig.w7.cc/strategy-id"
	RestartAnnotation        = "cloudconfig.w7.cc/restarted-at"
	AppliedRevisionAnnotation = "cloudconfig.w7.cc/applied-revision"
)

// CloudConfigSpec 描述配置中心配置的期望状态。
type CloudConfigSpec struct {
	// Name 是展示给用户看的配置名称。
	Name string `json:"name,omitempty"`
	// Items 是当前配置自身维护的配置项列表。
	Items []ConfigItem `json:"items,omitempty"`
	// Inherit 指定当前配置继承的底层配置；为空表示不继承其他配置。
	Inherit *ConfigInherit `json:"inherit,omitempty"`
	// Strategies 是将当前配置应用到工作负载的部署策略列表。
	Strategies []DeployStrategy `json:"strategies,omitempty"`
}

// ConfigItem 表示一条可按版本生效的配置项。
type ConfigItem struct {
	// Version 是配置项所属版本；为空时表示公共配置项。
	Version string `json:"version,omitempty"`
	// Name 是配置项名称，部署为环境变量或 ConfigMap key 时使用。
	Name string `json:"name,omitempty"`
	// Value 是配置项的值；允许为空字符串。
	Value string `json:"value,omitempty"`
	// Remark 是配置项备注，仅用于展示和说明。
	Remark string `json:"remark,omitempty"`
}

// ConfigInherit 描述当前配置继承的上游配置及其版本。
type ConfigInherit struct {
	// Namespace 是被继承配置所在命名空间；为空时默认使用当前配置命名空间。
	Namespace string `json:"namespace,omitempty"`
	// ConfigName 是被继承的 CloudConfig 资源名称。
	ConfigName string `json:"configName,omitempty"`
	// Version 是被继承配置的版本；为空时继承公共配置项。
	Version string `json:"version,omitempty"`
}

// DeployStrategy 描述一条配置部署策略。
type DeployStrategy struct {
	// ID 是部署策略唯一标识，用于记录应用状态和生成关联资源名称。
	ID string `json:"id,omitempty"`
	// Type 是部署类型；env 表示环境变量，file 表示配置文件挂载。
	Type string `json:"type,omitempty"`
	// Target 是部署策略作用的目标工作负载和容器。
	Target TargetRef `json:"target,omitempty"`
	// MountPath 是配置文件类型的容器内挂载路径；环境变量类型不使用。
	MountPath string `json:"mountPath,omitempty"`
	// AutoDeploy 表示配置更新后是否由控制器自动执行该部署策略。
	AutoDeploy bool `json:"autoDeploy,omitempty"`
	// LastSelectedVersion 记录用户上一次应用或开启自动部署时选择的配置版本。
	LastSelectedVersion string `json:"lastSelectedVersion,omitempty"`
}

// TargetRef 描述配置部署的目标容器。
type TargetRef struct {
	// Namespace 是目标工作负载所在命名空间。
	Namespace string `json:"namespace,omitempty"`
	// Kind 是目标工作负载类型，支持 Deployment、StatefulSet、DaemonSet。
	Kind string `json:"kind,omitempty"`
	// Name 是目标工作负载名称。
	Name string `json:"name,omitempty"`
	// Container 是目标工作负载 PodTemplate 中的容器名称。
	Container string `json:"container,omitempty"`
	// Group 是面板应用分组名称，用于前端展示目标应用归属。
	Group string `json:"group,omitempty"`
}

// CloudConfigStatus 描述配置中心配置的实际状态。
type CloudConfigStatus struct {
	// CreatedAt 是配置中心记录的创建时间。
	CreatedAt metav1.Time `json:"createdAt,omitempty"`
	// UpdatedAt 是配置内容或继承来源最近一次变化的时间。
	UpdatedAt metav1.Time `json:"updatedAt,omitempty"`
	// RecentUpdated 表示配置是否处于 24 小时内更新状态。
	RecentUpdated bool `json:"recentUpdated,omitempty"`
	// Revision 是根据 spec 计算出的配置版本指纹，用于判断部署策略是否已应用最新配置。
	Revision string `json:"revision,omitempty"`
	// LastApplied 记录每条部署策略最近一次应用结果。
	LastApplied []ApplyStatus `json:"lastApplied,omitempty"`
	// Conditions 记录控制器处理配置时产生的状态条件。
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ApplyStatus 描述一条部署策略的最近一次应用状态。
type ApplyStatus struct {
	// StrategyID 是对应 DeployStrategy 的 ID。
	StrategyID string `json:"strategyId,omitempty"`
	// Version 是本次应用时选择的配置版本；为空表示公共配置。
	Version string `json:"version,omitempty"`
	// Revision 是本次应用的配置版本指纹。
	Revision string `json:"revision,omitempty"`
	// AppliedAt 是本次应用发生的时间。
	AppliedAt metav1.Time `json:"appliedAt,omitempty"`
	// Success 表示本次应用是否成功。
	Success bool `json:"success"`
	// Error 记录本次应用失败时的错误信息。
	Error string `json:"error,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type CloudConfig struct {
	// TypeMeta 保存 Kubernetes 资源的 apiVersion 和 kind。
	metav1.TypeMeta `json:",inline"`
	// ObjectMeta 保存 Kubernetes 资源名称、命名空间、标签、注解等元数据。
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec 是用户期望的配置中心配置内容。
	Spec CloudConfigSpec `json:"spec,omitempty"`
	// Status 是控制器维护的配置状态和部署状态。
	Status CloudConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type CloudConfigList struct {
	// TypeMeta 保存列表资源的 apiVersion 和 kind。
	metav1.TypeMeta `json:",inline"`
	// ListMeta 保存列表分页和资源版本等元数据。
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items 是 CloudConfig 资源列表。
	Items []CloudConfig `json:"items"`
}
