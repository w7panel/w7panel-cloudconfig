package configcenter

import (
	"context"
	"fmt"
	"strconv"
	"time"

	cloudv1 "github.com/w7panel/w7panel-cloudconfig/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type ApplyResult struct {
	ItemCount     int    `json:"itemCount"`
	ConfigMapName string `json:"configMapName"`
	Revision      string `json:"revision"`
}

func ApplyStrategy(ctx context.Context, client ctrlclient.Client, config *cloudv1.CloudConfig, lookup func(namespace, name string) (*cloudv1.CloudConfig, bool), strategy cloudv1.DeployStrategy, version string) (*ApplyResult, error) {
	items, err := ResolveItems(config, lookup, version)
	if err != nil {
		return nil, err
	}
	data := ItemsToData(items)
	revision := Revision(config)
	cmName := StrategyConfigMapName(config, strategy.ID)
	if err := upsertConfigMap(ctx, client, strategy.Target.Namespace, cmName, strategy.ID, data); err != nil {
		return nil, err
	}
	if err := patchWorkload(ctx, client, strategy, cmName, revision); err != nil {
		return nil, err
	}
	return &ApplyResult{ItemCount: len(items), ConfigMapName: cmName, Revision: revision}, nil
}

func upsertConfigMap(ctx context.Context, client ctrlclient.Client, namespace, name, strategyID string, data map[string]string) error {
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	_, err := controllerutil.CreateOrPatch(ctx, client, cm, func() error {
		if cm.Labels == nil {
			cm.Labels = map[string]string{}
		}
		cm.Labels[cloudv1.DeployConfigMapLabel] = "true"
		cm.Labels[cloudv1.DeployConfigStrategyID] = strategyID
		cm.Data = data
		return nil
	})
	return err
}

func patchWorkload(ctx context.Context, client ctrlclient.Client, strategy cloudv1.DeployStrategy, configMapName, revision string) error {
	switch strategy.Target.Kind {
	case "deployments", "deployment", "Deployment":
		obj := &appsv1.Deployment{}
		if err := client.Get(ctx, types.NamespacedName{Namespace: strategy.Target.Namespace, Name: strategy.Target.Name}, obj); err != nil {
			return err
		}
		original := obj.DeepCopy()
		if err := mutatePodTemplate(&obj.Spec.Template, strategy, configMapName, revision); err != nil {
			return err
		}
		return client.Patch(ctx, obj, ctrlclient.MergeFrom(original))
	case "statefulsets", "statefulset", "StatefulSet":
		obj := &appsv1.StatefulSet{}
		if err := client.Get(ctx, types.NamespacedName{Namespace: strategy.Target.Namespace, Name: strategy.Target.Name}, obj); err != nil {
			return err
		}
		original := obj.DeepCopy()
		if err := mutatePodTemplate(&obj.Spec.Template, strategy, configMapName, revision); err != nil {
			return err
		}
		return client.Patch(ctx, obj, ctrlclient.MergeFrom(original))
	case "daemonsets", "daemonset", "DaemonSet":
		obj := &appsv1.DaemonSet{}
		if err := client.Get(ctx, types.NamespacedName{Namespace: strategy.Target.Namespace, Name: strategy.Target.Name}, obj); err != nil {
			return err
		}
		original := obj.DeepCopy()
		if err := mutatePodTemplate(&obj.Spec.Template, strategy, configMapName, revision); err != nil {
			return err
		}
		return client.Patch(ctx, obj, ctrlclient.MergeFrom(original))
	default:
		return fmt.Errorf("unsupported workload kind %q", strategy.Target.Kind)
	}
}

func mutatePodTemplate(template *corev1.PodTemplateSpec, strategy cloudv1.DeployStrategy, configMapName, revision string) error {
	containerIndex := -1
	for i := range template.Spec.Containers {
		if template.Spec.Containers[i].Name == strategy.Target.Container {
			containerIndex = i
			break
		}
	}
	if containerIndex < 0 {
		return fmt.Errorf("target container %q not found", strategy.Target.Container)
	}
	container := &template.Spec.Containers[containerIndex]
	volumeName := VolumeName(strategy.ID)

	filteredEnvFrom := container.EnvFrom[:0]
	for _, envFrom := range container.EnvFrom {
		if envFrom.ConfigMapRef != nil && envFrom.ConfigMapRef.Name == configMapName {
			continue
		}
		filteredEnvFrom = append(filteredEnvFrom, envFrom)
	}
	container.EnvFrom = filteredEnvFrom

	filteredMounts := container.VolumeMounts[:0]
	for _, mount := range container.VolumeMounts {
		if mount.Name == volumeName {
			continue
		}
		filteredMounts = append(filteredMounts, mount)
	}
	container.VolumeMounts = filteredMounts

	filteredVolumes := template.Spec.Volumes[:0]
	for _, volume := range template.Spec.Volumes {
		if volume.Name == volumeName {
			continue
		}
		filteredVolumes = append(filteredVolumes, volume)
	}
	template.Spec.Volumes = filteredVolumes

	if strategy.Type == StrategyTypeFile {
		template.Spec.Volumes = append(template.Spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
			}},
		})
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: strategy.MountPath,
			ReadOnly:  true,
		})
	} else {
		container.EnvFrom = append([]corev1.EnvFromSource{{
			ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: configMapName}},
		}}, container.EnvFrom...)
	}

	if template.Annotations == nil {
		template.Annotations = map[string]string{}
	}
	template.Annotations[cloudv1.RestartAnnotation] = strconv.FormatInt(time.Now().UnixNano(), 10)
	template.Annotations[cloudv1.AppliedRevisionAnnotation] = revision
	return nil
}

func IgnoreNotFound(err error) error {
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}
