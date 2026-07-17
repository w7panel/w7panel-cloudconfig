package controllers

import (
	"context"
	"strings"
	"testing"
	"time"

	cloudv1 "github.com/w7panel/w7panel-cloudconfig/api/v1alpha1"
	"github.com/w7panel/w7panel-cloudconfig/pkg/configcenter"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAutoDeployFailureUsesBackoff(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := cloudv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	strategy := cloudv1.DeployStrategy{
		ID: "auto", Type: configcenter.StrategyTypeEnv, AutoDeploy: true,
		Target: cloudv1.TargetRef{Namespace: "default", Kind: "Deployment", Name: "demo", Container: "missing"},
	}
	cfg := &cloudv1.CloudConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "app"},
		Spec:       cloudv1.CloudConfigSpec{Name: "app", Items: []cloudv1.ConfigItem{{Name: "KEY", Value: "value"}}, Strategies: []cloudv1.DeployStrategy{strategy}},
		Status:     cloudv1.CloudConfigStatus{Revision: "revision", UpdatedAt: metav1.Unix(100, 0), RecentUpdated: true},
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "web"}}}}},
	}
	client := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&cloudv1.CloudConfig{}).WithObjects(cfg, deployment).Build()
	stored := &cloudv1.CloudConfig{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "app"}, stored); err != nil {
		t.Fatal(err)
	}
	reconciler := &CloudConfigReconciler{Client: client, Scheme: scheme}
	firstAttempt := time.Unix(200, 0)
	wait, err := reconciler.runAutoDeploy(context.Background(), stored, firstAttempt)
	if err != nil {
		t.Fatal(err)
	}
	if wait != time.Minute {
		t.Fatalf("got retry %s, want 1m", wait)
	}
	status, ok := configcenter.StrategyLastStatus(stored, strategy.ID)
	if !ok || status.Success || status.FailureCount != 1 || !strings.Contains(status.Error, "missing") {
		t.Fatalf("unexpected failed status: %#v", status)
	}
	wait, err = reconciler.runAutoDeploy(context.Background(), stored, firstAttempt.Add(30*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if wait != 30*time.Second {
		t.Fatalf("got retry %s during backoff, want 30s", wait)
	}
	status, _ = configcenter.StrategyLastStatus(stored, strategy.ID)
	if status.FailureCount != 1 {
		t.Fatalf("expected skipped retry to preserve failure count, got %#v", status)
	}
	wait, err = reconciler.runAutoDeploy(context.Background(), stored, firstAttempt.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if wait != 2*time.Minute {
		t.Fatalf("got second retry %s, want 2m", wait)
	}
	status, _ = configcenter.StrategyLastStatus(stored, strategy.ID)
	if status.FailureCount != 2 {
		t.Fatalf("expected second failure count, got %#v", status)
	}
	stored.Status.UpdatedAt = metav1.NewTime(firstAttempt.Add(90 * time.Second))
	wait, err = reconciler.runAutoDeploy(context.Background(), stored, firstAttempt.Add(90*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	status, _ = configcenter.StrategyLastStatus(stored, strategy.ID)
	if wait != time.Minute || status.FailureCount != 1 {
		t.Fatalf("expected config change to retry immediately and reset backoff, wait=%s status=%#v", wait, status)
	}
}

func TestAutoDeploySuccessUpdatesWorkloadAndStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := cloudv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	strategy := cloudv1.DeployStrategy{
		ID: "auto", Type: configcenter.StrategyTypeEnv, AutoDeploy: true, LastSelectedVersion: "prod",
		Target: cloudv1.TargetRef{Namespace: "default", Kind: "Deployment", Name: "demo", Container: "web"},
	}
	cfg := &cloudv1.CloudConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "app"},
		Spec: cloudv1.CloudConfigSpec{
			Name:       "app",
			Items:      []cloudv1.ConfigItem{{Name: "KEY", Value: "public"}, {Version: "prod", Name: "KEY", Value: "prod"}},
			Strategies: []cloudv1.DeployStrategy{strategy},
		},
		Status: cloudv1.CloudConfigStatus{Revision: "revision", UpdatedAt: metav1.Unix(100, 0), RecentUpdated: true},
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "web"}}}}},
	}
	client := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&cloudv1.CloudConfig{}).WithObjects(cfg, deployment).Build()
	stored := &cloudv1.CloudConfig{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "app"}, stored); err != nil {
		t.Fatal(err)
	}
	reconciler := &CloudConfigReconciler{Client: client, Scheme: scheme}
	if wait, err := reconciler.runAutoDeploy(context.Background(), stored, time.Unix(200, 0)); err != nil || wait != 0 {
		t.Fatalf("run auto deploy wait=%s err=%v", wait, err)
	}
	status, ok := configcenter.StrategyLastStatus(stored, strategy.ID)
	if !ok || !status.Success || status.FailureCount != 0 || !status.NextRetryAt.IsZero() || status.Version != "prod" {
		t.Fatalf("unexpected success status: %#v", status)
	}
	updated := &appsv1.Deployment{}
	if err := client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "demo"}, updated); err != nil {
		t.Fatal(err)
	}
	if len(updated.Spec.Template.Spec.Containers[0].EnvFrom) != 1 {
		t.Fatalf("expected generated envFrom, got %#v", updated.Spec.Template.Spec.Containers[0].EnvFrom)
	}
}

func TestTouchDescendantsRecurses(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := cloudv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	base := &cloudv1.CloudConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "base"},
		Spec:       cloudv1.CloudConfigSpec{Name: "base"},
	}
	child := &cloudv1.CloudConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "child"},
		Spec:       cloudv1.CloudConfigSpec{Name: "child", Inherit: &cloudv1.ConfigInherit{ConfigName: "base"}},
	}
	grandchild := &cloudv1.CloudConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "grandchild"},
		Spec:       cloudv1.CloudConfigSpec{Name: "grandchild", Inherit: &cloudv1.ConfigInherit{ConfigName: "child"}},
	}
	client := ctrlclientfake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&cloudv1.CloudConfig{}).
		WithObjects(base, child, grandchild).
		Build()
	reconciler := &CloudConfigReconciler{Client: client, Scheme: scheme}
	now := metav1.Unix(200, 0)
	if err := reconciler.touchDescendants(context.Background(), base, now); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"child", "grandchild"} {
		cfg := &cloudv1.CloudConfig{}
		if err := client.Get(context.Background(), types.NamespacedName{Name: name}, cfg); err != nil {
			t.Fatal(err)
		}
		if !cfg.Status.RecentUpdated || !cfg.Status.UpdatedAt.Equal(&now) {
			t.Fatalf("expected %s to be touched at %s, got %#v", name, now, cfg.Status)
		}
	}
}

func TestReconcileDoesNotTouchDescendantsWithoutDataChange(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := cloudv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	base := &cloudv1.CloudConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "base"},
		Spec:       cloudv1.CloudConfigSpec{Name: "base"},
	}
	base.Status = cloudv1.CloudConfigStatus{
		CreatedAt: metav1.Unix(100, 0),
		UpdatedAt: metav1.Unix(100, 0),
		Revision:  configcenter.Revision(base),
	}
	child := &cloudv1.CloudConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "child"},
		Spec:       cloudv1.CloudConfigSpec{Name: "child", Inherit: &cloudv1.ConfigInherit{ConfigName: "base"}},
	}
	child.Status = cloudv1.CloudConfigStatus{
		CreatedAt: metav1.Unix(100, 0),
		UpdatedAt: metav1.Unix(100, 0),
		Revision:  configcenter.Revision(child),
	}
	client := ctrlclientfake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&cloudv1.CloudConfig{}).
		WithObjects(base, child).
		Build()
	reconciler := &CloudConfigReconciler{Client: client, Scheme: scheme}
	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "base"}})
	if err != nil {
		t.Fatal(err)
	}
	updated := &cloudv1.CloudConfig{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "child"}, updated); err != nil {
		t.Fatal(err)
	}
	if !updated.Status.UpdatedAt.Equal(&child.Status.UpdatedAt) {
		t.Fatalf("expected unchanged reconcile not to touch child, got %s want %s", updated.Status.UpdatedAt, child.Status.UpdatedAt)
	}
}
