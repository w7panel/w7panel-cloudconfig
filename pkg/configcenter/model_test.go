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
