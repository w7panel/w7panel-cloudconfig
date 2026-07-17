package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	cloudv1 "github.com/w7panel/w7panel-cloudconfig/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCreateConfigIsClusterScopedAndValidatesInheritance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := cloudv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	base := &cloudv1.CloudConfig{ObjectMeta: metav1.ObjectMeta{Name: "base"}, Spec: cloudv1.CloudConfigSpec{Name: "base"}}
	client := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&cloudv1.CloudConfig{}).WithObjects(base).Build()
	server := &Server{Client: client}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/cloudconfig-api/v1/configs", strings.NewReader(`{
		"metadata":{"namespace":"should-be-ignored","name":"child"},
		"spec":{"name":"child","inherit":{"configName":"base"},"items":[{"name":"KEY","value":"value"}]}
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	server.createConfig(ctx)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create returned %d: %s", recorder.Code, recorder.Body.String())
	}
	created := &cloudv1.CloudConfig{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "child"}, created); err != nil {
		t.Fatal(err)
	}
	if created.Namespace != "" || created.Spec.Inherit == nil || created.Spec.Inherit.ConfigName != "base" {
		t.Fatalf("unexpected cluster-scoped config: %#v", created)
	}

	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/cloudconfig-api/v1/configs", strings.NewReader(`{"metadata":{"name":"broken"},"spec":{"name":"broken","inherit":{"configName":"missing"}}}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	server.createConfig(ctx)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("missing inheritance returned %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestManualApplyFailurePersistsSelectionAndStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := cloudv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cfg := &cloudv1.CloudConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "app"},
		Spec: cloudv1.CloudConfigSpec{
			Name: "app", Items: []cloudv1.ConfigItem{{Name: "KEY", Value: "value"}},
			Strategies: []cloudv1.DeployStrategy{{
				ID: "strategy", Type: "env",
				Target: cloudv1.TargetRef{Namespace: "default", Kind: "Deployment", Name: "demo", Container: "missing"},
			}},
		},
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "web"}}}}},
	}
	client := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&cloudv1.CloudConfig{}).WithObjects(cfg, deployment).Build()
	server := &Server{Client: client}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "name", Value: "app"}, {Key: "strategy", Value: "strategy"}}
	ctx.Request = httptest.NewRequest(http.MethodPost, "/cloudconfig-api/v1/configs/app/strategies/strategy/apply", strings.NewReader(`{"version":"prod","autoDeploy":true}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	server.applyStrategy(ctx)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("failed apply returned %d: %s", recorder.Code, recorder.Body.String())
	}
	updated := &cloudv1.CloudConfig{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "app"}, updated); err != nil {
		t.Fatal(err)
	}
	strategy := updated.Spec.Strategies[0]
	if !strategy.AutoDeploy || strategy.LastSelectedVersion != "prod" {
		t.Fatalf("expected apply choices to persist, got %#v", strategy)
	}
	if len(updated.Status.LastApplied) != 1 || updated.Status.LastApplied[0].Success || updated.Status.LastApplied[0].FailureCount != 1 || updated.Status.LastApplied[0].NextRetryAt.IsZero() {
		t.Fatalf("expected failed apply status, got %#v", updated.Status.LastApplied)
	}
}
