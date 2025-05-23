/*
Copyright 2024 zncdatadev.

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
	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	s3v1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/s3/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DolphinCommonPropertiesName     = "common.properties"
	DolphinConfigPath               = "/opt/dolphinscheduler/conf"
	LogbackPropertiesFileName       = "logback-spring.xml"
	ApplicationServerConfigFileName = "application.yaml"

	DbInitImage              = "apache/dolphinscheduler-tools:3.2.1"
	MaxLogFileSize           = "10Mi"
	ConsoleConversionPattern = "%d{ISO8601} - %-5p [%t:%C{1}@%L] - %m%n"
)

const (
	ConfigVolumeName              = "common-config"
	LogbackVolumeName             = "logback"
	WorkerDataVolumeName          = "worker-data"
	LoggingVolumeName             = "log"
	LdapBindCredintialsVolumeName = "ldap-bind-credentials"
)

const (
	MasterPortName       = "port"
	MasterActualPortName = "actual-port"
	MasterPort           = 5678
	MasterActualPort     = 5679

	WorkerPortName       = "port"
	WorkerActualPortName = "actual-port"
	WorkerPort           = 1234
	WorkerActualPort     = 1235

	ApiPortName       = "port"
	ApiPythonPortName = "python-port"
	ApiPort           = 12345
	ApiPythonPort     = 25333

	AlerterPortName       = "port"
	AlerterActualPortName = "actual-port"
	AlerterPort           = 50052
	AlerterActualPort     = 50053
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// DolphinschedulerCluster is the Schema for the dolphinschedulerclusters API
type DolphinschedulerCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DolphinschedulerClusterSpec `json:"spec,omitempty"`
	Status status.Status               `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DolphinschedulerClusterList contains a list of DolphinschedulerCluster
type DolphinschedulerClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DolphinschedulerCluster `json:"items"`
}

// DolphinschedulerClusterSpec defines the desired state of DolphinschedulerCluster
type DolphinschedulerClusterSpec struct {
	// +kubebuilder:validation:Optional
	// +default:value={"repo": "quay.io/zncdatadev", "pullPolicy": "IfNotPresent"}
	Image *ImageSpec `json:"image"`

	// +kubebuilder:validation:Optional
	ClusterConfig *ClusterConfigSpec `json:"clusterConfig,omitempty"`

	// +kubebuilder:validation:Optional
	ClusterOperationSpec *commonsv1alpha1.ClusterOperationSpec `json:"clusterOperation,omitempty"`

	// +kubebuilder:validation:Required
	Master *RoleSpec `json:"master,omitempty"`

	// +kubebuilder:validation:Required
	Worker *RoleSpec `json:"worker,omitempty"`

	// +kubebuilder:validation:Required
	Alerter *RoleSpec `json:"alerter,omitempty"`

	// +kubebuilder:validation:Required
	Api *RoleSpec `json:"api,omitempty"`
}

type ClusterConfigSpec struct {

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="cluster.local"
	ClusterDomain string `json:"clusterDomain,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="example.com"
	IngressHost string `json:"ingressHost,omitempty"`

	// +kubebuilder:validation:Optional
	VectorAggregatorConfigMapName string `json:"vectorAggregatorConfigMapName,omitempty"`

	// +kubebuilder:validation:Required
	ZookeeperConfigMapName string `json:"zookeeperConfigMapName,omitempty"`

	// +kubebuilder:validation:Optional
	S3 *s3v1alpha1.S3BucketSpec `json:"s3,omitempty"`

	// +kubebuilder:validation:Required
	Database *DatabaseSpec `json:"database,omitempty"`

	// +kubebuilder:validation:Optional
	Authentication []AuthenticationSpec `json:"authentication,omitempty"`
}

type DatabaseSpec struct {
	// +kubebuilder:validation:Required
	ConnectionString string `json:"connectionString,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:default="h2"
	DatabaseType string `json:"databaseType,omitempty"`

	// +kubebuilder:validation:Optional
	CredentialsSecret string `json:"credentialsSecret,omitempty"`
}

type RoleSpec struct {
	// +kubebuilder:validation:Optional
	Config *ConfigSpec `json:"config,omitempty"`

	// +kubebuilder:validation:Optional
	RoleGroups map[string]RoleGroupSpec `json:"roleGroups,omitempty"`

	// +kubebuilder:validation:Optional
	RoleConfig *commonsv1alpha1.RoleConfigSpec `json:"roleConfig,omitempty"`

	*commonsv1alpha1.OverridesSpec `json:",inline"`
}

type RoleGroupSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=1
	Replicas *int32 `json:"replicas,omitempty"`

	// +kubebuilder:validation:Optional
	Config *ConfigSpec `json:"config,omitempty"`

	*commonsv1alpha1.OverridesSpec `json:",inline"`
}
type ConfigSpec struct {
	*commonsv1alpha1.RoleGroupConfigSpec `json:",inline"`
}

func init() {
	SchemeBuilder.Register(&DolphinschedulerCluster{}, &DolphinschedulerClusterList{})
}
