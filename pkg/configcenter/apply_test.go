package configcenter

import (
	"context"
	"testing"

	cloudv1 "github.com/w7panel/w7panel-cloudconfig/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestApplyStrategyEnv(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := cloudv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cfg := &cloudv1.CloudConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "app-config", Namespace: "default"},
		Spec: cloudv1.CloudConfigSpec{
			Name:  "app",
			Items: []cloudv1.ConfigItem{{Name: "MYSQL_HOST", Value: "mysql"}},
		},
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "demo"}},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "web"}}},
			},
		},
	}
	client := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(cfg, deployment).Build()
	strategy := cloudv1.DeployStrategy{
		ID:   "strategy-env",
		Type: StrategyTypeEnv,
		Target: cloudv1.TargetRef{
			Namespace: "default",
			Kind:      "Deployment",
			Name:      "demo",
			Container: "web",
		},
	}
	result, err := ApplyStrategy(context.Background(), client, cfg, func(namespace, name string) (*cloudv1.CloudConfig, bool) {
		return nil, false
	}, strategy, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.ItemCount != 1 {
		t.Fatalf("expected 1 item, got %d", result.ItemCount)
	}
	updated := &appsv1.Deployment{}
	if err := client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "demo"}, updated); err != nil {
		t.Fatal(err)
	}
	envFrom := updated.Spec.Template.Spec.Containers[0].EnvFrom
	if len(envFrom) != 1 || envFrom[0].ConfigMapRef == nil || envFrom[0].ConfigMapRef.Name == "" {
		t.Fatalf("expected envFrom configMapRef, got %#v", envFrom)
	}
	cm := &corev1.ConfigMap{}
	if err := client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: envFrom[0].ConfigMapRef.Name}, cm); err != nil {
		t.Fatal(err)
	}
	if cm.Data["MYSQL_HOST"] != "mysql" {
		t.Fatalf("expected configmap data, got %#v", cm.Data)
	}
}
