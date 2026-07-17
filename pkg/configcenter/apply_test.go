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
		ObjectMeta: metav1.ObjectMeta{Name: "app-config"},
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
				Spec: corev1.PodSpec{Containers: []corev1.Container{{
					Name: "web",
					Env:  []corev1.EnvVar{{Name: "MYSQL_HOST", Value: "app-owned"}},
					EnvFrom: []corev1.EnvFromSource{{
						ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "app-env"}},
					}},
				}}},
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
	result, err := ApplyStrategy(context.Background(), client, cfg, func(name string) (*cloudv1.CloudConfig, bool) {
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
	if len(envFrom) != 2 || envFrom[0].ConfigMapRef == nil || envFrom[0].ConfigMapRef.Name == "" {
		t.Fatalf("expected envFrom configMapRef, got %#v", envFrom)
	}
	if envFrom[1].ConfigMapRef == nil || envFrom[1].ConfigMapRef.Name != "app-env" {
		t.Fatalf("expected existing app envFrom to keep precedence, got %#v", envFrom)
	}
	cm := &corev1.ConfigMap{}
	if err := client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: envFrom[0].ConfigMapRef.Name}, cm); err != nil {
		t.Fatal(err)
	}
	if cm.Data["MYSQL_HOST"] != "mysql" {
		t.Fatalf("expected configmap data, got %#v", cm.Data)
	}
	if env := updated.Spec.Template.Spec.Containers[0].Env; len(env) != 1 || env[0].Value != "app-owned" {
		t.Fatalf("expected explicit app env to remain unchanged, got %#v", env)
	}
	if _, err := ApplyStrategy(context.Background(), client, cfg, func(name string) (*cloudv1.CloudConfig, bool) { return nil, false }, strategy, ""); err != nil {
		t.Fatal(err)
	}
	if err := client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "demo"}, updated); err != nil {
		t.Fatal(err)
	}
	if got := len(updated.Spec.Template.Spec.Containers[0].EnvFrom); got != 2 {
		t.Fatalf("expected idempotent envFrom sources, got %d", got)
	}
}

func TestApplyStrategyFile(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := cloudv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cfg := &cloudv1.CloudConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "app-config"},
		Spec: cloudv1.CloudConfigSpec{
			Name:  "app",
			Items: []cloudv1.ConfigItem{{Name: "config.yaml", Value: "debug: false"}},
		},
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "demo"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "web"}}},
			},
		},
	}
	client := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(cfg, deployment).Build()
	strategy := cloudv1.DeployStrategy{
		ID:        "strategy-file",
		Type:      StrategyTypeFile,
		MountPath: "/app/config",
		Target: cloudv1.TargetRef{
			Namespace: "default",
			Kind:      "Deployment",
			Name:      "demo",
			Container: "web",
		},
	}
	result, err := ApplyStrategy(context.Background(), client, cfg, func(name string) (*cloudv1.CloudConfig, bool) {
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
	if len(updated.Spec.Template.Spec.Volumes) != 1 || updated.Spec.Template.Spec.Volumes[0].ConfigMap == nil {
		t.Fatalf("expected configmap volume, got %#v", updated.Spec.Template.Spec.Volumes)
	}
	mounts := updated.Spec.Template.Spec.Containers[0].VolumeMounts
	if len(mounts) != 1 || mounts[0].MountPath != "/app/config" || !mounts[0].ReadOnly {
		t.Fatalf("expected read-only mount at /app/config, got %#v", mounts)
	}
	cm := &corev1.ConfigMap{}
	if err := client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: updated.Spec.Template.Spec.Volumes[0].ConfigMap.Name}, cm); err != nil {
		t.Fatal(err)
	}
	if cm.Data["config.yaml"] != "debug: false" {
		t.Fatalf("expected configmap data, got %#v", cm.Data)
	}
	if _, err := ApplyStrategy(context.Background(), client, cfg, func(name string) (*cloudv1.CloudConfig, bool) { return nil, false }, strategy, ""); err != nil {
		t.Fatal(err)
	}
	if err := client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "demo"}, updated); err != nil {
		t.Fatal(err)
	}
	if len(updated.Spec.Template.Spec.Volumes) != 1 || len(updated.Spec.Template.Spec.Containers[0].VolumeMounts) != 1 {
		t.Fatalf("expected idempotent file mount, got volumes=%#v mounts=%#v", updated.Spec.Template.Spec.Volumes, updated.Spec.Template.Spec.Containers[0].VolumeMounts)
	}
}

func TestApplyStrategySupportsStatefulSetAndDaemonSet(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := cloudv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "stateful", Namespace: "default"},
		Spec:       appsv1.StatefulSetSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "web"}}}}},
	}
	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "daemon", Namespace: "default"},
		Spec:       appsv1.DaemonSetSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "web"}}}}},
	}
	client := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(statefulSet, daemonSet).Build()
	for _, target := range []struct{ kind, name string }{{"StatefulSet", "stateful"}, {"DaemonSet", "daemon"}} {
		strategy := cloudv1.DeployStrategy{
			ID: "strategy-" + target.name, Type: StrategyTypeEnv,
			Target: cloudv1.TargetRef{Namespace: "default", Kind: target.kind, Name: target.name, Container: "web"},
		}
		if err := patchWorkload(context.Background(), client, strategy, "generated-config", "revision"); err != nil {
			t.Fatalf("patch %s: %v", target.kind, err)
		}
	}
	updatedStateful := &appsv1.StatefulSet{}
	if err := client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "stateful"}, updatedStateful); err != nil {
		t.Fatal(err)
	}
	updatedDaemon := &appsv1.DaemonSet{}
	if err := client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "daemon"}, updatedDaemon); err != nil {
		t.Fatal(err)
	}
	if len(updatedStateful.Spec.Template.Spec.Containers[0].EnvFrom) != 1 || len(updatedDaemon.Spec.Template.Spec.Containers[0].EnvFrom) != 1 {
		t.Fatal("expected both workload kinds to receive envFrom")
	}
}

func TestApplyStrategyEnvRejectsInvalidEnvName(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := cloudv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cfg := &cloudv1.CloudConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "app-config"},
		Spec: cloudv1.CloudConfigSpec{
			Name:  "app",
			Items: []cloudv1.ConfigItem{{Name: "mysql.host", Value: "mysql"}},
		},
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "demo"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "web"}}},
			},
		},
	}
	client := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(cfg, deployment).Build()
	_, err := ApplyStrategy(context.Background(), client, cfg, func(name string) (*cloudv1.CloudConfig, bool) {
		return nil, false
	}, cloudv1.DeployStrategy{
		ID:   "strategy-env",
		Type: StrategyTypeEnv,
		Target: cloudv1.TargetRef{
			Namespace: "default",
			Kind:      "Deployment",
			Name:      "demo",
			Container: "web",
		},
	}, "")
	if err == nil {
		t.Fatal("expected invalid env name error")
	}
}
