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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DolphinCommonPropertiesName = "common.properties"
	LogbackPropertiesFileName   = "logback-spring.xml"
	ConsoleConversionPattern    = "%d{ISO8601} - %-5p [%t:%C{1}@%L] - %m%n"
)

const (
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

	MetricsPortName = "metrics"
)

// RoleMaster, RoleWorker, RoleApi and RoleAlert are the canonical role names of the generic
// cluster spec (GetSpec's Roles map keys). RoleAlert deliberately stays "alert" — the historical
// role name every resource name ("<cluster>-alert-<group>") and the e2e suites are pinned to —
// even though the spec field is named "alerter" and the container "alerter-server".
const (
	RoleMaster = "master"
	RoleWorker = "worker"
	RoleApi    = "api"
	RoleAlert  = "alert"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// DolphinschedulerCluster is the Schema for the dolphinschedulerclusters API
type DolphinschedulerCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DolphinschedulerClusterSpec   `json:"spec,omitempty"`
	Status DolphinschedulerClusterStatus `json:"status,omitempty"`
}

// DolphinschedulerClusterStatus is the observed state, carried entirely by the framework's
// generic cluster status (conditions, per-role-group entries, observedGeneration).
type DolphinschedulerClusterStatus struct {
	commonsv1alpha1.GenericClusterStatus `json:",inline"`
}

// GetSpec implements common.ClusterInterface by adapting the product spec to the framework's
// generic cluster spec. The typed role fields map to Roles keys "master", "worker", "api" and
// "alert" (NOT "alerter": resource names derive from the role key and are e2e-pinned).
func (r *DolphinschedulerCluster) GetSpec() *commonsv1alpha1.GenericClusterSpec {
	spec := &commonsv1alpha1.GenericClusterSpec{
		ClusterOperation: r.Spec.ClusterOperationSpec,
		Image:            r.Spec.Image.toCommonsImage(),
	}

	roles := map[string]commonsv1alpha1.RoleSpec{}
	for name, role := range map[string]*RoleSpec{
		RoleMaster: r.Spec.Master,
		RoleWorker: r.Spec.Worker,
		RoleApi:    r.Spec.Api,
		RoleAlert:  r.Spec.Alerter,
	} {
		if role == nil {
			continue
		}
		roles[name] = role.toCommonsRole()
	}
	if len(roles) > 0 {
		spec.Roles = roles
	}
	return spec
}

// GetStatus implements common.ClusterInterface. The framework mutates the returned pointer.
func (r *DolphinschedulerCluster) GetStatus() *commonsv1alpha1.GenericClusterStatus {
	return &r.Status.GenericClusterStatus
}

// VectorAggregatorConfigMapName implements reconciler.VectorAggregatorProvider: when a role
// group enables the Vector agent, the framework resolves the aggregator address from this
// ConfigMap and renders vector.yaml into the role group ConfigMap. "" means unset.
func (r *DolphinschedulerCluster) VectorAggregatorConfigMapName() string {
	if r.Spec.ClusterConfig == nil {
		return ""
	}
	return r.Spec.ClusterConfig.VectorAggregatorConfigMapName
}

// toCommonsImage adapts the product image spec (which keeps the historical `koobdoopVersion`
// json tag for CRD compatibility) to the commons ImageSpec.
func (s *ImageSpec) toCommonsImage() *commonsv1alpha1.ImageSpec {
	if s == nil {
		return nil
	}
	img := &commonsv1alpha1.ImageSpec{
		Custom:          s.Custom,
		Repo:            s.Repo,
		ProductVersion:  s.ProductVersion,
		KubedoopVersion: s.KubedoopVersion,
		PullSecretName:  s.PullSecretName,
	}
	if s.PullPolicy != nil {
		img.PullPolicy = *s.PullPolicy
	}
	return img
}

// toCommonsRole adapts a product RoleSpec (inlined OverridesSpec + ConfigSpec) to the commons
// RoleSpec with its flattened override fields.
func (r *RoleSpec) toCommonsRole() commonsv1alpha1.RoleSpec {
	role := commonsv1alpha1.RoleSpec{
		RoleConfig: r.RoleConfig,
	}
	if r.Config != nil {
		role.Config = r.Config.RoleGroupConfigSpec
	}
	if r.OverridesSpec != nil {
		role.ConfigOverrides = r.ConfigOverrides
		role.EnvOverrides = r.EnvOverrides
		role.CliOverrides = r.CliOverrides
		role.PodOverrides = r.PodOverrides
	}
	if len(r.RoleGroups) > 0 {
		groups := make(map[string]commonsv1alpha1.RoleGroupSpec, len(r.RoleGroups))
		for name, group := range r.RoleGroups {
			groups[name] = group.toCommonsRoleGroup()
		}
		role.RoleGroups = groups
	}
	return role
}

// toCommonsRoleGroup adapts a product RoleGroupSpec to the commons RoleGroupSpec.
func (g *RoleGroupSpec) toCommonsRoleGroup() commonsv1alpha1.RoleGroupSpec {
	group := commonsv1alpha1.RoleGroupSpec{
		Replicas: g.Replicas,
	}
	if g.Config != nil {
		group.Config = g.Config.RoleGroupConfigSpec
	}
	if g.OverridesSpec != nil {
		group.ConfigOverrides = g.ConfigOverrides
		group.EnvOverrides = g.EnvOverrides
		group.CliOverrides = g.CliOverrides
		group.PodOverrides = g.PodOverrides
	}
	return group
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

	// +kubebuilder:validation:Required
	ClusterConfig *ClusterConfigSpec `json:"clusterConfig"`

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
	// +kubebuilder:validation:MinLength=1
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
	// +kubebuilder:validation:MinLength=1
	ConnectionString string `json:"connectionString,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:default="h2"
	DatabaseType string `json:"databaseType,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinLength=1
	CredentialsSecret string `json:"credentialsSecret,omitempty"`
}

type RoleSpec struct {
	// +kubebuilder:validation:Optional
	Config *ConfigSpec `json:"config,omitempty"`

	// Role-group names become Kubernetes resource-name and label segments.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinProperties=1
	// +kubebuilder:validation:MaxProperties=256
	// +kubebuilder:validation:XValidation:rule=`self.all(k, size(k) <= 63 && k.matches('^[a-z0-9]([-a-z0-9]*[a-z0-9])?$'))`,message=`each role group name must be a lowercase RFC 1123 label (lowercase alphanumerics and '-', starting and ending with an alphanumeric, at most 63 characters)`
	RoleGroups map[string]RoleGroupSpec `json:"roleGroups,omitempty"`

	// +kubebuilder:validation:Optional
	RoleConfig *commonsv1alpha1.RoleConfigSpec `json:"roleConfig,omitempty"`

	*commonsv1alpha1.OverridesSpec `json:",inline"`
}

type RoleGroupSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
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
