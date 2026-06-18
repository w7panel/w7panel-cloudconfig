package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (in *CloudConfig) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(CloudConfig)
	*out = *in
	out.ObjectMeta = *in.ObjectMeta.DeepCopy()
	out.Spec = *in.Spec.DeepCopy()
	out.Status = *in.Status.DeepCopy()
	return out
}

func (in *CloudConfigList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(CloudConfigList)
	*out = *in
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		out.Items = make([]CloudConfig, len(in.Items))
		for i := range in.Items {
			out.Items[i] = *in.Items[i].DeepCopy()
		}
	}
	return out
}

func (in *CloudConfig) DeepCopy() *CloudConfig {
	if in == nil {
		return nil
	}
	out := new(CloudConfig)
	*out = *in
	out.ObjectMeta = *in.ObjectMeta.DeepCopy()
	out.Spec = *in.Spec.DeepCopy()
	out.Status = *in.Status.DeepCopy()
	return out
}

func (in *CloudConfigSpec) DeepCopy() *CloudConfigSpec {
	if in == nil {
		return nil
	}
	out := new(CloudConfigSpec)
	*out = *in
	if in.Items != nil {
		out.Items = append([]ConfigItem(nil), in.Items...)
	}
	if in.Inherit != nil {
		out.Inherit = new(ConfigInherit)
		*out.Inherit = *in.Inherit
	}
	if in.Strategies != nil {
		out.Strategies = append([]DeployStrategy(nil), in.Strategies...)
	}
	return out
}

func (in *CloudConfigStatus) DeepCopy() *CloudConfigStatus {
	if in == nil {
		return nil
	}
	out := new(CloudConfigStatus)
	*out = *in
	if in.LastApplied != nil {
		out.LastApplied = append([]ApplyStatus(nil), in.LastApplied...)
	}
	if in.Conditions != nil {
		out.Conditions = append([]metav1.Condition(nil), in.Conditions...)
	}
	return out
}
