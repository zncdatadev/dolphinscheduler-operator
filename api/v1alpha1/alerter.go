package v1alpha1

import corev1 "k8s.io/api/core/v1"

type AlerterSpec struct {

	// +kubebuilder:validation:Required
	Image AlerterImageSpec `json:"image,omitempty"`

	// +kubebuilder:validation:Optional
	Config *ConfigSpec `json:"config,omitempty"`

	// +kubebuilder:validation:Optional
	RoleGroups map[string]*RoleGroupSpec `json:"roleGroups,omitempty"`

	// +kubebuilder:validation:Optional
	PodDisruptionBudget *PodDisruptionBudgetSpec `json:"podDisruptionBudget,omitempty"`

	// +kubebuilder:validation:Optional
	CommandArgsOverrides []string `json:"commandArgsOverrides,omitempty"`

	// +kubebuilder:validation:Optional
	ConfigOverrides *ConfigOverridesSpec `json:"configOverrides,omitempty"`

	// +kubebuilder:validation:Optional
	EnvOverrides map[string]string `json:"envOverrides,omitempty"`
}
type AlerterImageSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=apache/dolphinscheduler-alert-server
	Repository string `json:"repository,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="3.2.1"
	Tag string `json:"tag,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=IfNotPresent
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`
}
