package controllers

import (
	"context"
	"time"

	cloudv1 "github.com/w7panel/w7panel-cloudconfig/api/v1alpha1"
	"github.com/w7panel/w7panel-cloudconfig/pkg/configcenter"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type CloudConfigReconciler struct {
	ctrlclient.Client
	Scheme *runtime.Scheme
	Now    func() time.Time
}

func (r *CloudConfigReconciler) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *CloudConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	cfg := &cloudv1.CloudConfig{}
	if err := r.Get(ctx, req.NamespacedName, cfg); err != nil {
		return ctrl.Result{}, configcenter.IgnoreNotFound(err)
	}

	changedStatus := false
	dataChanged := false
	revision := configcenter.Revision(cfg)
	nowTime := r.now()
	now := metav1.NewTime(nowTime)
	if cfg.Status.CreatedAt.IsZero() {
		cfg.Status.CreatedAt = cfg.CreationTimestamp
		if cfg.Status.CreatedAt.IsZero() {
			cfg.Status.CreatedAt = now
		}
		changedStatus = true
	}
	if cfg.Status.Revision != revision {
		cfg.Status.Revision = revision
		cfg.Status.UpdatedAt = now
		cfg.Status.RecentUpdated = true
		changedStatus = true
		dataChanged = true
	}
	if !cfg.Status.UpdatedAt.IsZero() && nowTime.Sub(cfg.Status.UpdatedAt.Time) >= 24*time.Hour && cfg.Status.RecentUpdated {
		cfg.Status.RecentUpdated = false
		changedStatus = true
	}

	if changedStatus {
		if err := r.Status().Update(ctx, cfg); err != nil {
			return ctrl.Result{}, err
		}
	}

	if dataChanged {
		if err := r.touchDescendants(ctx, cfg, now); err != nil {
			return ctrl.Result{}, err
		}
	}
	requeueAfter := time.Hour
	if cfg.Status.RecentUpdated {
		nextRetry, err := r.runAutoDeploy(ctx, cfg, nowTime)
		if err != nil {
			return ctrl.Result{}, err
		}
		if nextRetry > 0 && nextRetry < requeueAfter {
			requeueAfter = nextRetry
		}
	}
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

func (r *CloudConfigReconciler) touchDescendants(ctx context.Context, root *cloudv1.CloudConfig, now metav1.Time) error {
	return r.touchDescendantsByName(ctx, root.Name, now, map[string]bool{root.Name: true})
}

func (r *CloudConfigReconciler) touchDescendantsByName(ctx context.Context, parentName string, now metav1.Time, visited map[string]bool) error {
	list := &cloudv1.CloudConfigList{}
	if err := r.List(ctx, list); err != nil {
		return err
	}
	for i := range list.Items {
		item := &list.Items[i]
		if item.Spec.Inherit == nil || item.Spec.Inherit.ConfigName != parentName {
			continue
		}
		key := item.Name
		if visited[key] {
			continue
		}
		visited[key] = true
		if item.Status.UpdatedAt.Before(&now) {
			item.Status.UpdatedAt = now
			item.Status.RecentUpdated = true
			if err := r.Status().Update(ctx, item); err != nil {
				return err
			}
		}
		if err := r.touchDescendantsByName(ctx, item.Name, now, visited); err != nil {
			return err
		}
	}
	return nil
}

func (r *CloudConfigReconciler) runAutoDeploy(ctx context.Context, cfg *cloudv1.CloudConfig, now time.Time) (time.Duration, error) {
	changed := false
	var nextRetry time.Duration
	for _, strategy := range cfg.Spec.Strategies {
		if !strategy.AutoDeploy || !configcenter.StrategyStale(cfg, strategy.ID) {
			continue
		}
		previous, hasPrevious := configcenter.StrategyLastStatus(cfg, strategy.ID)
		strategyRevision := configcenter.StrategyRevision(strategy)
		sameFailure := hasPrevious && !previous.Success && previous.Revision == cfg.Status.Revision && previous.StrategyRevision == strategyRevision && !cfg.Status.UpdatedAt.After(previous.AppliedAt.Time)
		if sameFailure && !previous.NextRetryAt.IsZero() && previous.NextRetryAt.After(now) {
			wait := previous.NextRetryAt.Sub(now)
			if nextRetry == 0 || wait < nextRetry {
				nextRetry = wait
			}
			continue
		}
		result, err := configcenter.ApplyStrategy(ctx, r.Client, cfg, r.lookupConfig(ctx), strategy, strategy.LastSelectedVersion)
		status := cloudv1.ApplyStatus{
			StrategyID:       strategy.ID,
			StrategyRevision: strategyRevision,
			Version:          strategy.LastSelectedVersion,
			Revision:         cfg.Status.Revision,
			AppliedAt:        metav1.NewTime(now),
			Success:          err == nil,
		}
		if err != nil {
			status.Error = err.Error()
			status.FailureCount = 1
			if sameFailure {
				status.FailureCount = previous.FailureCount + 1
			}
			delay := configcenter.AutoDeployRetryDelay(status.FailureCount)
			status.NextRetryAt = metav1.NewTime(now.Add(delay))
			if nextRetry == 0 || delay < nextRetry {
				nextRetry = delay
			}
		} else if result != nil {
			status.Revision = result.Revision
		}
		cfg.Status.LastApplied = upsertApplyStatus(cfg.Status.LastApplied, status)
		changed = true
	}
	if changed {
		if err := r.Status().Update(ctx, cfg); err != nil {
			return 0, err
		}
	}
	return nextRetry, nil
}

func (r *CloudConfigReconciler) lookupConfig(ctx context.Context) func(name string) (*cloudv1.CloudConfig, bool) {
	return func(name string) (*cloudv1.CloudConfig, bool) {
		cfg := &cloudv1.CloudConfig{}
		if err := r.Get(ctx, types.NamespacedName{Name: name}, cfg); err != nil {
			return nil, false
		}
		return cfg, true
	}
}

func upsertApplyStatus(list []cloudv1.ApplyStatus, item cloudv1.ApplyStatus) []cloudv1.ApplyStatus {
	for i := range list {
		if list[i].StrategyID == item.StrategyID {
			list[i] = item
			return list
		}
	}
	return append(list, item)
}

func (r *CloudConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&cloudv1.CloudConfig{}).Complete(r)
}
