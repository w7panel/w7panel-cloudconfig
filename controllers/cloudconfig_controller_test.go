package controllers

import (
	"context"
	"testing"

	cloudv1 "github.com/w7panel/w7panel-cloudconfig/api/v1alpha1"
	"github.com/w7panel/w7panel-cloudconfig/pkg/configcenter"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestTouchDescendantsRecurses(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := cloudv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	base := &cloudv1.CloudConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "base", Namespace: "default"},
		Spec:       cloudv1.CloudConfigSpec{Name: "base"},
	}
	child := &cloudv1.CloudConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "child", Namespace: "default"},
		Spec:       cloudv1.CloudConfigSpec{Name: "child", Inherit: &cloudv1.ConfigInherit{ConfigName: "base"}},
	}
	grandchild := &cloudv1.CloudConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "grandchild", Namespace: "default"},
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
		if err := client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: name}, cfg); err != nil {
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
		ObjectMeta: metav1.ObjectMeta{Name: "base", Namespace: "default"},
		Spec:       cloudv1.CloudConfigSpec{Name: "base"},
	}
	base.Status = cloudv1.CloudConfigStatus{
		CreatedAt: metav1.Unix(100, 0),
		UpdatedAt: metav1.Unix(100, 0),
		Revision:  configcenter.Revision(base),
	}
	child := &cloudv1.CloudConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "child", Namespace: "default"},
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
	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "base"}})
	if err != nil {
		t.Fatal(err)
	}
	updated := &cloudv1.CloudConfig{}
	if err := client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "child"}, updated); err != nil {
		t.Fatal(err)
	}
	if !updated.Status.UpdatedAt.Equal(&child.Status.UpdatedAt) {
		t.Fatalf("expected unchanged reconcile not to touch child, got %s want %s", updated.Status.UpdatedAt, child.Status.UpdatedAt)
	}
}
