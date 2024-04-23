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

const DolphinCommonPropertiesName = "common.properties"

const (
	MasterPortName       = "master-port"
	MasterActualPortName = "master-actual-port"
	MasterPort           = 5678
	MasterActualPort     = 5679

	WorkerPortName       = "worker-port"
	WorkerActualPortName = "worker-actual-port"
	WorkerPort           = 1234
	WorkerActualPort     = 1235

	ApiPortName       = "api-port"
	ApiPythonPortName = "api-python-port"
	ApiPort           = 12345
	ApiPythonPort     = 25333

	AlerterPortName       = "alerter-port"
	AlerterActualPortName = "alerter-actual-port"
	AlerterPort           = 50052
	AlerterActualPort     = 50053
)

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
	// +kubebuilder:validation:Required
	ClusterConfigSpec *ClusterConfigSpec `json:"clusterConfig,omitempty"`

	// +kubebuilder:validation:Required
	Master *MasterSpec `json:"master,omitempty"`

	// +kubebuilder:validation:Required
	Worker *WorkerSpec `json:"worker,omitempty"`

	// +kubebuilder:validation:Required
	Alerter *AlerterSpec `json:"alerter,omitempty"`

	// +kubebuilder:validation:Required
	Api *ApiSpec `json:"api,omitempty"`
}

type ClusterConfigSpec struct {

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="cluster.local"
	ClusterDomain string `json:"clusterDomain,omitempty"`

	// +kubebuilder:validation:Required
	ZookeeperDiscoveryZNode string `json:"zookeeperDiscoveryZNode,omitempty"`

	// +kubebuilder:validation:Required
	S3Bucket *S3BucketSpec `json:"s3Bucket,omitempty"`

	// +kubebuilder:validation:Required
	Database *DatabaseSpec `json:"database,omitempty"`
}

type DatabaseSpec struct {
	// +kubebuilder:validation=Optional
	Reference string `json:"reference"`

	// +kubebuilder:validation=Optional
	Inline *DatabaseInlineSpec `json:"inline,omitempty"`
}

// DatabaseInlineSpec defines the inline database spec.
type DatabaseInlineSpec struct {
	// +kubebuilder:validation:Enum=mysql;postgres
	// +kubebuilder:default="postgres"
	Driver string `json:"driver,omitempty"`

	// +kubebuilder:validation=Optional
	// +kubebuilder:default="hive"
	DatabaseName string `json:"databaseName,omitempty"`

	// +kubebuilder:validation=Optional
	// +kubebuilder:default="hive"
	Username string `json:"username,omitempty"`

	// +kubebuilder:validation=Optional
	// +kubebuilder:default="hive"
	Password string `json:"password,omitempty"`

	// +kubebuilder:validation=Required
	Host string `json:"host,omitempty"`

	// +kubebuilder:validation=Optional
	// +kubebuilder:default=5432
	Port int32 `json:"port,omitempty"`
}

type S3BucketSpec struct {
	// S3 bucket name with S3Bucket
	// +kubebuilder:validation=Optional
	Reference *string `json:"reference"`

	// +kubebuilder:validation=Optional
	Inline *S3BucketInlineSpec `json:"inline,omitempty"`

	// +kubebuilder:validation=Optional
	// +kubebuilder:default=20
	MaxConnect int `json:"maxConnect"`

	// +kubebuilder:validation=Optional
	PathStyleAccess bool `json:"pathStyle_access"`
}

type S3BucketInlineSpec struct {

	// +kubeBuilder:validation=Required
	Bucket string `json:"bucket"`

	// +kubebuilder:validation=Optional
	// +kubebuilder:default="us-east-1"
	Region string `json:"region,omitempty"`

	// +kubebuilder:validation=Required
	Endpoints string `json:"endpoints"`

	// +kubebuilder:validation=Optional
	// +kubebuilder:default=false
	SSL bool `json:"ssl,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=false
	PathStyle bool `json:"pathStyle,omitempty"`

	// +kubebuilder:validation=Optional
	AccessKey string `json:"accessKey,omitempty"`

	// +kubebuilder:validation=Optional
	SecretKey string `json:"secretKey,omitempty"`
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
	// +kubebuilder:default="2Gi"
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
