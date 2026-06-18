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

type CloudConfigSpec struct {
	Name       string           `json:"name,omitempty"`
	Items      []ConfigItem     `json:"items,omitempty"`
	Inherit    *ConfigInherit   `json:"inherit,omitempty"`
	Strategies []DeployStrategy `json:"strategies,omitempty"`
}

type ConfigItem struct {
	Version string `json:"version,omitempty"`
	Name    string `json:"name,omitempty"`
	Value   string `json:"value,omitempty"`
	Remark  string `json:"remark,omitempty"`
}

type ConfigInherit struct {
	Namespace  string `json:"namespace,omitempty"`
	ConfigName string `json:"configName,omitempty"`
	Version    string `json:"version,omitempty"`
}

type DeployStrategy struct {
	ID                  string    `json:"id,omitempty"`
	Type                string    `json:"type,omitempty"`
	Target              TargetRef `json:"target,omitempty"`
	MountPath           string    `json:"mountPath,omitempty"`
	AutoDeploy          bool      `json:"autoDeploy,omitempty"`
	LastSelectedVersion string    `json:"lastSelectedVersion,omitempty"`
}

type TargetRef struct {
	Namespace string `json:"namespace,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Name      string `json:"name,omitempty"`
	Container string `json:"container,omitempty"`
	Group     string `json:"group,omitempty"`
}

type CloudConfigStatus struct {
	CreatedAt     metav1.Time   `json:"createdAt,omitempty"`
	UpdatedAt     metav1.Time   `json:"updatedAt,omitempty"`
	RecentUpdated bool          `json:"recentUpdated,omitempty"`
	Revision      string        `json:"revision,omitempty"`
	LastApplied   []ApplyStatus `json:"lastApplied,omitempty"`
	Conditions    []metav1.Condition `json:"conditions,omitempty"`
}

type ApplyStatus struct {
	StrategyID string      `json:"strategyId,omitempty"`
	Version    string      `json:"version,omitempty"`
	Revision   string      `json:"revision,omitempty"`
	AppliedAt  metav1.Time `json:"appliedAt,omitempty"`
	Success    bool        `json:"success"`
	Error      string      `json:"error,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type CloudConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CloudConfigSpec   `json:"spec,omitempty"`
	Status CloudConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type CloudConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CloudConfig `json:"items"`
}
