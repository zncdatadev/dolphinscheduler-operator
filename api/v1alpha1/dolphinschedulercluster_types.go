/*
Copyright 2024 zncdata-labs.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"github.com/zncdata-labs/operator-go/pkg/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DolphinschedulerCluster is the Schema for the dolphinschedulerclusters API
type DolphinschedulerCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DolphinschedulerClusterSpec `json:"spec,omitempty"`
	Status status.Status               `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DolphinschedulerClusterList contains a list of DolphinschedulerCluster
type DolphinschedulerClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DolphinschedulerCluster `json:"items"`
}

// DolphinschedulerClusterSpec defines the desired state of DolphinschedulerCluster
type DolphinschedulerClusterSpec struct {
	// +kubebuilder:validation:required
	Image ImageSpec `json:"image,omitempty"`

	// +kubebuilder:validation:Required
	ClusterConfigSpec *ClusterConfigSpec `json:"clusterConfig,omitempty"`

	// +kubebuilder:validation:Required
	MasterSpec *MasterSpec `json:"master,omitempty"`

	// +kubebuilder:validation:Required
	WorkerSpec *WorkerSpec `json:"worker,omitempty"`

	// +kubebuilder:validation:Required
	AlerterSpec *AlerterSpec `json:"alerter,omitempty"`

	// +kubebuilder:validation:Required
	Api *ApiSpec `json:"api,omitempty"`
}

type ImageSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=bitnami/kafka
	Repository string `json:"repository,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="3.7.0-debian-12-r2"
	Tag string `json:"tag,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=IfNotPresent
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`
}

type ClusterConfigSpec struct {

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="cluster.local"
	ClusterDomain string `json:"clusterDomain,omitempty"`

	// +kubebuilder:validation:required
	ZookeeperDiscoveryZNode string `json:"zookeeperDiscoveryZNode,omitempty"`
}

type ContainerLoggingSpec struct {
	// +kubebuilder:validation:Optional
	Logging *LoggingConfigSpec `json:"logging,omitempty"`
}

type ConfigOverridesSpec struct {
}

type RoleGroupSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=1
	Replicas int32 `json:"replicas,omitempty"`

	// +kubebuilder:validation:Required
	Config *ConfigSpec `json:"config,omitempty"`

	// +kubebuilder:validation:Optional
	CommandArgsOverrides []string `json:"commandArgsOverrides,omitempty"`

	// +kubebuilder:validation:Optional
	ConfigOverrides *ConfigOverridesSpec `json:"configOverrides,omitempty"`

	// +kubebuilder:validation:Optional
	EnvOverrides map[string]string `json:"envOverrides,omitempty"`
}

type ConfigSpec struct {
	// +kubebuilder:validation:Optional
	Resources *ResourcesSpec `json:"resources,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="cluster-internal"
	ListenerClass string `json:"listenerClass,omitempty"`

	// +kubebuilder:validation:Optional
	SecurityContext *corev1.PodSecurityContext `json:"securityContext"`

	// +kubebuilder:validation:Optional
	Affinity *corev1.Affinity `json:"affinity"`

	// +kubebuilder:validation:Optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// +kubebuilder:validation:Optional
	Tolerations []corev1.Toleration `json:"tolerations"`

	// +kubebuilder:validation:Optional
	PodDisruptionBudget *PodDisruptionBudgetSpec `json:"podDisruptionBudget,omitempty"`

	// +kubebuilder:validation:Optional
	StorageClass string `json:"storageClass,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default="8Gi"
	StorageSize string `json:"storageSize,omitempty"`

	// +kubebuilder:validation:Optional
	ExtraEnv map[string]string `json:"extraEnv,omitempty"`

	// +kubebuilder:validation:Optional
	ExtraSecret map[string]string `json:"extraSecret,omitempty"`

	// +kubebuilder:validation:Optional
	Logging *ContainerLoggingSpec `json:"logging,omitempty"`
}
type PodDisruptionBudgetSpec struct {
	// +kubebuilder:validation:Optional
	MinAvailable int32 `json:"minAvailable,omitempty"`

	// +kubebuilder:validation:Optional
	MaxUnavailable int32 `json:"maxUnavailable,omitempty"`
}

func init() {
	SchemeBuilder.Register(&DolphinschedulerCluster{}, &DolphinschedulerClusterList{})
}
