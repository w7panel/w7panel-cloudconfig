package configcenter

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	cloudv1 "github.com/w7panel/w7panel-cloudconfig/api/v1alpha1"
)

const (
	StrategyTypeEnv  = "env"
	StrategyTypeFile = "file"
)

type ResolvedItem struct {
	cloudv1.ConfigItem `json:",inline"`
	Source             string `json:"source,omitempty"`
	SourceNamespace    string `json:"sourceNamespace,omitempty"`
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
	for i := range config.Spec.Items {
		config.Spec.Items[i].Version = strings.TrimSpace(config.Spec.Items[i].Version)
		config.Spec.Items[i].Name = strings.TrimSpace(config.Spec.Items[i].Name)
	}
	for i := range config.Spec.Strategies {
		if config.Spec.Strategies[i].Type == "" {
			config.Spec.Strategies[i].Type = StrategyTypeEnv
		}
		if config.Spec.Strategies[i].Target.Namespace == "" {
			config.Spec.Strategies[i].Target.Namespace = config.Namespace
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
		if strategy.Target.Kind == "" || strategy.Target.Name == "" || strategy.Target.Container == "" {
			return fmt.Errorf("strategies[%d].target kind, name and container are required", i)
		}
		if strategy.Type == StrategyTypeFile && strings.TrimSpace(strategy.MountPath) == "" {
			return fmt.Errorf("strategies[%d].mountPath is required for file strategy", i)
		}
	}
	return nil
}

func ResolveItems(root *cloudv1.CloudConfig, lookup func(namespace, name string) (*cloudv1.CloudConfig, bool), version string) ([]ResolvedItem, error) {
	return resolveItems(root, lookup, version, map[string]bool{})
}

func resolveItems(root *cloudv1.CloudConfig, lookup func(namespace, name string) (*cloudv1.CloudConfig, bool), version string, stack map[string]bool) ([]ResolvedItem, error) {
	if root == nil {
		return nil, nil
	}
	key := root.Namespace + "/" + root.Name
	if stack[key] {
		return nil, fmt.Errorf("circular inherit detected at %s", key)
	}
	stack[key] = true
	defer delete(stack, key)

	var inherited []ResolvedItem
	if root.Spec.Inherit != nil && root.Spec.Inherit.ConfigName != "" {
		ns := root.Spec.Inherit.Namespace
		if ns == "" {
			ns = root.Namespace
		}
		parent, ok := lookup(ns, root.Spec.Inherit.ConfigName)
		if !ok {
			return nil, fmt.Errorf("inherited config %s/%s not found", ns, root.Spec.Inherit.ConfigName)
		}
		items, err := resolveItems(parent, lookup, root.Spec.Inherit.Version, stack)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			item.Source = "inherit"
			item.SourceNamespace = parent.Namespace
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
		if item.Name == "" {
			continue
		}
		if item.Version != "" && item.Version != version {
			continue
		}
		if _, ok := merged[item.Name]; !ok {
			order = append(order, item.Name)
		}
		merged[item.Name] = ResolvedItem{
			ConfigItem:      item,
			Source:          "self",
			SourceNamespace: root.Namespace,
			SourceName:      root.Name,
			SourceTitle:     root.Spec.Name,
		}
	}

	result := make([]ResolvedItem, 0, len(order))
	for _, name := range order {
		result = append(result, merged[name])
	}
	return result, nil
}

func AvailableVersions(configs ...*cloudv1.CloudConfig) []string {
	seen := map[string]bool{}
	for _, cfg := range configs {
		if cfg == nil {
			continue
		}
		for _, item := range cfg.Spec.Items {
			if item.Version != "" {
				seen[item.Version] = true
			}
		}
		if cfg.Spec.Inherit != nil && cfg.Spec.Inherit.Version != "" {
			seen[cfg.Spec.Inherit.Version] = true
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
	data, _ := json.Marshal(config.Spec)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:16]
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
