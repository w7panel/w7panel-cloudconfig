package configcenter

import (
	"testing"

	cloudv1 "github.com/w7panel/w7panel-cloudconfig/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResolveItemsInheritAndOverride(t *testing.T) {
	parent := &cloudv1.CloudConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "base", Namespace: "default"},
		Spec: cloudv1.CloudConfigSpec{
			Name: "base",
			Items: []cloudv1.ConfigItem{
				{Name: "MYSQL_HOST", Value: "mysql"},
				{Version: "prod", Name: "REDIS_HOST", Value: "redis-prod"},
			},
		},
	}
	child := &cloudv1.CloudConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Spec: cloudv1.CloudConfigSpec{
			Name:    "app",
			Inherit: &cloudv1.ConfigInherit{ConfigName: "base", Version: "prod"},
			Items: []cloudv1.ConfigItem{
				{Name: "MYSQL_HOST", Value: "mysql-app"},
				{Name: "APP_NAME", Value: "demo"},
			},
		},
	}
	items, err := ResolveItems(child, func(namespace, name string) (*cloudv1.CloudConfig, bool) {
		if namespace == "default" && name == "base" {
			return parent, true
		}
		return nil, false
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	data := ItemsToData(items)
	if data["MYSQL_HOST"] != "mysql-app" {
		t.Fatalf("expected child override, got %q", data["MYSQL_HOST"])
	}
	if data["REDIS_HOST"] != "redis-prod" {
		t.Fatalf("expected inherited selected version, got %q", data["REDIS_HOST"])
	}
	if data["APP_NAME"] != "demo" {
		t.Fatalf("expected child item, got %q", data["APP_NAME"])
	}
}

func TestResolveItemsCircularInherit(t *testing.T) {
	a := &cloudv1.CloudConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "default"},
		Spec:       cloudv1.CloudConfigSpec{Name: "a", Inherit: &cloudv1.ConfigInherit{ConfigName: "b"}},
	}
	b := &cloudv1.CloudConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "b", Namespace: "default"},
		Spec:       cloudv1.CloudConfigSpec{Name: "b", Inherit: &cloudv1.ConfigInherit{ConfigName: "a"}},
	}
	_, err := ResolveItems(a, func(namespace, name string) (*cloudv1.CloudConfig, bool) {
		if name == "a" {
			return a, true
		}
		if name == "b" {
			return b, true
		}
		return nil, false
	}, "")
	if err == nil {
		t.Fatal("expected circular inherit error")
	}
}

func TestResolveAllItemsIncludesAllSelfVersions(t *testing.T) {
	parent := &cloudv1.CloudConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "base", Namespace: "default"},
		Spec: cloudv1.CloudConfigSpec{
			Name: "base",
			Items: []cloudv1.ConfigItem{
				{Name: "BASE_PUBLIC", Value: "public"},
				{Version: "prod", Name: "BASE_PROD", Value: "prod"},
				{Version: "test", Name: "BASE_TEST", Value: "test"},
			},
		},
	}
	child := &cloudv1.CloudConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Spec: cloudv1.CloudConfigSpec{
			Name:    "app",
			Inherit: &cloudv1.ConfigInherit{ConfigName: "base", Version: "prod"},
			Items: []cloudv1.ConfigItem{
				{Name: "APP_PUBLIC", Value: "public"},
				{Version: "prod", Name: "APP_PROD", Value: "prod"},
				{Version: "test", Name: "APP_TEST", Value: "test"},
			},
		},
	}
	items, err := ResolveAllItems(child, func(namespace, name string) (*cloudv1.CloudConfig, bool) {
		if namespace == "default" && name == "base" {
			return parent, true
		}
		return nil, false
	})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, item := range items {
		seen[item.Source+":"+item.Name] = true
	}
	for _, key := range []string{"inherit:BASE_PUBLIC", "inherit:BASE_PROD", "self:APP_PUBLIC", "self:APP_PROD", "self:APP_TEST"} {
		if !seen[key] {
			t.Fatalf("expected %s in resolved all items, got %#v", key, seen)
		}
	}
	if seen["inherit:BASE_TEST"] {
		t.Fatalf("expected inherited items to respect selected version, got %#v", seen)
	}
}

func TestRevisionIgnoresDeployStrategies(t *testing.T) {
	cfg := &cloudv1.CloudConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Spec: cloudv1.CloudConfigSpec{
			Name:  "app",
			Items: []cloudv1.ConfigItem{{Name: "MYSQL_HOST", Value: "mysql"}},
		},
	}
	before := Revision(cfg)
	cfg.Spec.Strategies = []cloudv1.DeployStrategy{{
		ID:         "strategy-env",
		Type:       StrategyTypeEnv,
		AutoDeploy: true,
		Target:     cloudv1.TargetRef{Namespace: "default", Kind: "Deployment", Name: "demo", Container: "web"},
	}}
	if after := Revision(cfg); after != before {
		t.Fatalf("expected strategy-only change to keep revision %s, got %s", before, after)
	}
	cfg.Spec.Items[0].Value = "changed"
	if after := Revision(cfg); after == before {
		t.Fatalf("expected data change to alter revision")
	}
}

func TestStrategyStaleUsesUpdatedAt(t *testing.T) {
	updatedAt := metav1.Unix(200, 0)
	appliedAt := metav1.Unix(100, 0)
	cfg := &cloudv1.CloudConfig{
		Status: cloudv1.CloudConfigStatus{
			Revision:  "abc",
			UpdatedAt: updatedAt,
			LastApplied: []cloudv1.ApplyStatus{{
				StrategyID: "strategy-env",
				Revision:   "abc",
				AppliedAt:  appliedAt,
				Success:    true,
			}},
		},
	}
	if !StrategyStale(cfg, "strategy-env") {
		t.Fatal("expected strategy to be stale when applied before updatedAt")
	}
	cfg.Status.LastApplied[0].AppliedAt = metav1.Unix(300, 0)
	if StrategyStale(cfg, "strategy-env") {
		t.Fatal("expected strategy to be fresh when applied after updatedAt")
	}
}

func TestStrategyStaleUsesStrategyRevision(t *testing.T) {
	strategy := cloudv1.DeployStrategy{
		ID:        "strategy-file",
		Type:      StrategyTypeFile,
		MountPath: "/app/config",
		Target:    cloudv1.TargetRef{Namespace: "default", Kind: "Deployment", Name: "demo", Container: "web"},
	}
	cfg := &cloudv1.CloudConfig{
		Spec: cloudv1.CloudConfigSpec{Strategies: []cloudv1.DeployStrategy{strategy}},
		Status: cloudv1.CloudConfigStatus{
			Revision:  "abc",
			UpdatedAt: metav1.Unix(100, 0),
			LastApplied: []cloudv1.ApplyStatus{{
				StrategyID:       strategy.ID,
				StrategyRevision: StrategyRevision(strategy),
				Revision:         "abc",
				AppliedAt:        metav1.Unix(200, 0),
				Success:          true,
			}},
		},
	}
	if StrategyStale(cfg, strategy.ID) {
		t.Fatal("expected unchanged strategy to be fresh")
	}
	cfg.Spec.Strategies[0].MountPath = "/etc/config"
	if !StrategyStale(cfg, strategy.ID) {
		t.Fatal("expected strategy mount path change to be stale")
	}
}
