package configcenter

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"regexp"
	"sort"
	"strings"
	"time"

	cloudv1 "github.com/w7panel/w7panel-cloudconfig/api/v1alpha1"
)

const (
	StrategyTypeEnv  = "env"
	StrategyTypeFile = "file"
)

type ResolvedItem struct {
	cloudv1.ConfigItem `json:",inline"`
	Source             string `json:"source,omitempty"`
	SourceName         string `json:"sourceName,omitempty"`
	SourceTitle        string `json:"sourceTitle,omitempty"`
}

func NormalizeConfig(config *cloudv1.CloudConfig) {
	if config.Labels == nil {
		config.Labels = map[string]string{}
	}
	config.Labels[cloudv1.CloudConfigLabelManaged] = "true"
	if config.Spec.Name != "" {
		config.Labels[cloudv1.CloudConfigLabelName] = sanitizeLabel(config.Spec.Name)
	}
	versions := make([]string, 0, len(config.Spec.Versions))
	seenVersions := map[string]bool{}
	for _, version := range config.Spec.Versions {
		version = strings.TrimSpace(version)
		if version == "" || seenVersions[version] {
			continue
		}
		seenVersions[version] = true
		versions = append(versions, version)
	}
	config.Spec.Versions = versions
	for i := range config.Spec.Items {
		config.Spec.Items[i].Version = strings.TrimSpace(config.Spec.Items[i].Version)
		config.Spec.Items[i].Name = strings.TrimSpace(config.Spec.Items[i].Name)
	}
	for i := range config.Spec.Strategies {
		if config.Spec.Strategies[i].Type == "" {
			config.Spec.Strategies[i].Type = StrategyTypeEnv
		}
	}
}

func Validate(config *cloudv1.CloudConfig) error {
	if strings.TrimSpace(config.Spec.Name) == "" {
		return fmt.Errorf("name is required")
	}
	for i, item := range config.Spec.Items {
		if strings.TrimSpace(item.Name) == "" {
			return fmt.Errorf("items[%d].name is required", i)
		}
	}
	for i, strategy := range config.Spec.Strategies {
		if strategy.ID == "" {
			return fmt.Errorf("strategies[%d].id is required", i)
		}
		if strategy.Type != StrategyTypeEnv && strategy.Type != StrategyTypeFile {
			return fmt.Errorf("strategies[%d].type must be env or file", i)
		}
		if strategy.Target.Namespace == "" || strategy.Target.Kind == "" || strategy.Target.Name == "" || strategy.Target.Container == "" {
			return fmt.Errorf("strategies[%d].target namespace, kind, name and container are required", i)
		}
		if strategy.Type == StrategyTypeFile && strings.TrimSpace(strategy.MountPath) == "" {
			return fmt.Errorf("strategies[%d].mountPath is required for file strategy", i)
		}
	}
	return nil
}

func ValidateInheritance(config *cloudv1.CloudConfig, lookup func(name string) (*cloudv1.CloudConfig, bool)) error {
	if config == nil {
		return nil
	}
	rootKey := config.Name
	current := config
	seen := map[string]bool{rootKey: true}
	for current.Spec.Inherit != nil && current.Spec.Inherit.ConfigName != "" {
		key := current.Spec.Inherit.ConfigName
		if seen[key] {
			return fmt.Errorf("circular inherit detected at %s", key)
		}
		seen[key] = true
		parent, ok := lookup(current.Spec.Inherit.ConfigName)
		if !ok {
			return fmt.Errorf("inherited config %s not found", key)
		}
		current = parent
	}
	return nil
}

func ResolveItems(root *cloudv1.CloudConfig, lookup func(name string) (*cloudv1.CloudConfig, bool), version string) ([]ResolvedItem, error) {
	return resolveItems(root, lookup, version, map[string]bool{})
}

func ResolveAllItems(root *cloudv1.CloudConfig, lookup func(name string) (*cloudv1.CloudConfig, bool)) ([]ResolvedItem, error) {
	return resolveAllItems(root, lookup, map[string]bool{})
}

func resolveItems(root *cloudv1.CloudConfig, lookup func(name string) (*cloudv1.CloudConfig, bool), version string, stack map[string]bool) ([]ResolvedItem, error) {
	if root == nil {
		return nil, nil
	}
	key := root.Name
	if stack[key] {
		return nil, fmt.Errorf("circular inherit detected at %s", key)
	}
	stack[key] = true
	defer delete(stack, key)

	var inherited []ResolvedItem
	if root.Spec.Inherit != nil && root.Spec.Inherit.ConfigName != "" {
		parent, ok := lookup(root.Spec.Inherit.ConfigName)
		if !ok {
			return nil, fmt.Errorf("inherited config %s not found", root.Spec.Inherit.ConfigName)
		}
		items, err := resolveItems(parent, lookup, root.Spec.Inherit.Version, stack)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			item.Source = "inherit"
			item.SourceName = parent.Name
			item.SourceTitle = parent.Spec.Name
			inherited = append(inherited, item)
		}
	}

	merged := map[string]ResolvedItem{}
	order := []string{}
	for _, item := range inherited {
		if item.Name == "" {
			continue
		}
		if _, ok := merged[item.Name]; !ok {
			order = append(order, item.Name)
		}
		merged[item.Name] = item
	}
	for _, item := range root.Spec.Items {
		if item.Name == "" || item.Version != "" {
			continue
		}
		if _, ok := merged[item.Name]; !ok {
			order = append(order, item.Name)
		}
		merged[item.Name] = ResolvedItem{
			ConfigItem:  item,
			Source:      "self",
			SourceName:  root.Name,
			SourceTitle: root.Spec.Name,
		}
	}
	for _, item := range root.Spec.Items {
		if item.Name == "" {
			continue
		}
		if version == "" || item.Version != version {
			continue
		}
		if _, ok := merged[item.Name]; !ok {
			order = append(order, item.Name)
		}
		merged[item.Name] = ResolvedItem{
			ConfigItem:  item,
			Source:      "self",
			SourceName:  root.Name,
			SourceTitle: root.Spec.Name,
		}
	}

	result := make([]ResolvedItem, 0, len(order))
	for _, name := range order {
		result = append(result, merged[name])
	}
	return result, nil
}

func resolveAllItems(root *cloudv1.CloudConfig, lookup func(name string) (*cloudv1.CloudConfig, bool), stack map[string]bool) ([]ResolvedItem, error) {
	if root == nil {
		return nil, nil
	}
	key := root.Name
	if stack[key] {
		return nil, fmt.Errorf("circular inherit detected at %s", key)
	}
	stack[key] = true
	defer delete(stack, key)

	result := []ResolvedItem{}
	if root.Spec.Inherit != nil && root.Spec.Inherit.ConfigName != "" {
		parent, ok := lookup(root.Spec.Inherit.ConfigName)
		if !ok {
			return nil, fmt.Errorf("inherited config %s not found", root.Spec.Inherit.ConfigName)
		}
		items, err := resolveItems(parent, lookup, root.Spec.Inherit.Version, stack)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			item.Source = "inherit"
			item.SourceName = parent.Name
			item.SourceTitle = parent.Spec.Name
			result = append(result, item)
		}
	}

	for _, item := range root.Spec.Items {
		if item.Name == "" {
			continue
		}
		result = append(result, ResolvedItem{
			ConfigItem:  item,
			Source:      "self",
			SourceName:  root.Name,
			SourceTitle: root.Spec.Name,
		})
	}
	return result, nil
}

func AvailableVersions(configs ...*cloudv1.CloudConfig) []string {
	seen := map[string]bool{}
	for _, cfg := range configs {
		if cfg == nil {
			continue
		}
		for _, version := range cfg.Spec.Versions {
			if version != "" {
				seen[version] = true
			}
		}
		for _, item := range cfg.Spec.Items {
			if item.Version != "" {
				seen[item.Version] = true
			}
		}
	}
	result := make([]string, 0, len(seen))
	for version := range seen {
		result = append(result, version)
	}
	sort.Strings(result)
	return result
}

func Revision(config *cloudv1.CloudConfig) string {
	data, _ := json.Marshal(struct {
		Name     string                 `json:"name,omitempty"`
		Versions []string               `json:"versions,omitempty"`
		Items    []cloudv1.ConfigItem   `json:"items,omitempty"`
		Inherit  *cloudv1.ConfigInherit `json:"inherit,omitempty"`
	}{
		Name:     config.Spec.Name,
		Versions: config.Spec.Versions,
		Items:    config.Spec.Items,
		Inherit:  config.Spec.Inherit,
	})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:16]
}

func StrategyRevision(strategy cloudv1.DeployStrategy) string {
	data, _ := json.Marshal(map[string]any{
		"lastSelectedVersion": strategy.LastSelectedVersion,
		"mountPath":           strategy.MountPath,
		"target": map[string]string{
			"container": strategy.Target.Container,
			"group":     strategy.Target.Group,
			"kind":      strategy.Target.Kind,
			"name":      strategy.Target.Name,
			"namespace": strategy.Target.Namespace,
		},
		"type": strategy.Type,
	})
	hash := fnv.New32a()
	_, _ = hash.Write(data)
	return fmt.Sprintf("%x", hash.Sum32())
}

func StrategyAppliedStatus(config *cloudv1.CloudConfig, strategyID string) (cloudv1.ApplyStatus, bool) {
	if config == nil {
		return cloudv1.ApplyStatus{}, false
	}
	for _, item := range config.Status.LastApplied {
		if item.StrategyID == strategyID && item.Success {
			return item, true
		}
	}
	return cloudv1.ApplyStatus{}, false
}

func StrategyLastStatus(config *cloudv1.CloudConfig, strategyID string) (cloudv1.ApplyStatus, bool) {
	if config == nil {
		return cloudv1.ApplyStatus{}, false
	}
	for _, item := range config.Status.LastApplied {
		if item.StrategyID == strategyID {
			return item, true
		}
	}
	return cloudv1.ApplyStatus{}, false
}

func AutoDeployRetryDelay(failureCount int32) time.Duration {
	if failureCount < 1 {
		failureCount = 1
	}
	if failureCount >= 5 {
		return 15 * time.Minute
	}
	delay := time.Minute << (failureCount - 1)
	if delay > 15*time.Minute {
		return 15 * time.Minute
	}
	return delay
}

func StrategyStale(config *cloudv1.CloudConfig, strategyID string) bool {
	status, ok := StrategyAppliedStatus(config, strategyID)
	if !ok {
		return true
	}
	for _, strategy := range config.Spec.Strategies {
		if strategy.ID == strategyID && status.StrategyRevision != "" && status.StrategyRevision != StrategyRevision(strategy) {
			return true
		}
	}
	if config.Status.Revision != "" && status.Revision != "" && status.Revision != config.Status.Revision {
		return true
	}
	if !config.Status.UpdatedAt.IsZero() && (status.AppliedAt.IsZero() || status.AppliedAt.Before(&config.Status.UpdatedAt)) {
		return true
	}
	return false
}

func ItemsToData(items []ResolvedItem) map[string]string {
	data := map[string]string{}
	for _, item := range items {
		if item.Name == "" {
			continue
		}
		data[item.Name] = item.Value
	}
	return data
}

func StrategyConfigMapName(config *cloudv1.CloudConfig, strategyID string) string {
	base := strings.TrimPrefix(config.Name, "cloudconfig-")
	return dns1123(fmt.Sprintf("cc-%s-%s", base, strategyID), 63)
}

func VolumeName(strategyID string) string {
	return dns1123("cc-"+strategyID, 63)
}

func sanitizeLabel(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 63 {
		value = value[:63]
	}
	return value
}

func dns1123(value string, max int) string {
	value = strings.ToLower(value)
	value = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		value = "cloudconfig"
	}
	if len(value) > max {
		value = strings.Trim(value[:max], "-")
	}
	return value
}
