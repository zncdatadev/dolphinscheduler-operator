//go:build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	s3v1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/s3/v1alpha1"
	"k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AuthenticationSpec) DeepCopyInto(out *AuthenticationSpec) {
	*out = *in
	if in.Oidc != nil {
		in, out := &in.Oidc, &out.Oidc
		*out = new(OidcCredentialSecretSpec)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AuthenticationSpec.
func (in *AuthenticationSpec) DeepCopy() *AuthenticationSpec {
	if in == nil {
		return nil
	}
	out := new(AuthenticationSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterConfigSpec) DeepCopyInto(out *ClusterConfigSpec) {
	*out = *in
	if in.S3 != nil {
		in, out := &in.S3, &out.S3
		*out = new(s3v1alpha1.S3BucketSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.Database != nil {
		in, out := &in.Database, &out.Database
		*out = new(DatabaseSpec)
		**out = **in
	}
	if in.Authentication != nil {
		in, out := &in.Authentication, &out.Authentication
		*out = make([]AuthenticationSpec, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterConfigSpec.
func (in *ClusterConfigSpec) DeepCopy() *ClusterConfigSpec {
	if in == nil {
		return nil
	}
	out := new(ClusterConfigSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ConfigOverridesSpec) DeepCopyInto(out *ConfigOverridesSpec) {
	*out = *in
	if in.CommonProperties != nil {
		in, out := &in.CommonProperties, &out.CommonProperties
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ConfigOverridesSpec.
func (in *ConfigOverridesSpec) DeepCopy() *ConfigOverridesSpec {
	if in == nil {
		return nil
	}
	out := new(ConfigOverridesSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ConfigSpec) DeepCopyInto(out *ConfigSpec) {
	*out = *in
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = new(commonsv1alpha1.ResourcesSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.Affinity != nil {
		in, out := &in.Affinity, &out.Affinity
		*out = new(v1.Affinity)
		(*in).DeepCopyInto(*out)
	}
	if in.PodDisruptionBudget != nil {
		in, out := &in.PodDisruptionBudget, &out.PodDisruptionBudget
		*out = new(commonsv1alpha1.PodDisruptionBudgetSpec)
		**out = **in
	}
	if in.GracefulShutdownTimeout != nil {
		in, out := &in.GracefulShutdownTimeout, &out.GracefulShutdownTimeout
		*out = new(string)
		**out = **in
	}
	if in.Logging != nil {
		in, out := &in.Logging, &out.Logging
		*out = new(ContainerLoggingSpec)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ConfigSpec.
func (in *ConfigSpec) DeepCopy() *ConfigSpec {
	if in == nil {
		return nil
	}
	out := new(ConfigSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ContainerLoggingSpec) DeepCopyInto(out *ContainerLoggingSpec) {
	*out = *in
	if in.Logging != nil {
		in, out := &in.Logging, &out.Logging
		*out = new(commonsv1alpha1.LoggingConfigSpec)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ContainerLoggingSpec.
func (in *ContainerLoggingSpec) DeepCopy() *ContainerLoggingSpec {
	if in == nil {
		return nil
	}
	out := new(ContainerLoggingSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DatabaseSpec) DeepCopyInto(out *DatabaseSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DatabaseSpec.
func (in *DatabaseSpec) DeepCopy() *DatabaseSpec {
	if in == nil {
		return nil
	}
	out := new(DatabaseSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DolphinschedulerCluster) DeepCopyInto(out *DolphinschedulerCluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DolphinschedulerCluster.
func (in *DolphinschedulerCluster) DeepCopy() *DolphinschedulerCluster {
	if in == nil {
		return nil
	}
	out := new(DolphinschedulerCluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DolphinschedulerCluster) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DolphinschedulerClusterList) DeepCopyInto(out *DolphinschedulerClusterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]DolphinschedulerCluster, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DolphinschedulerClusterList.
func (in *DolphinschedulerClusterList) DeepCopy() *DolphinschedulerClusterList {
	if in == nil {
		return nil
	}
	out := new(DolphinschedulerClusterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DolphinschedulerClusterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DolphinschedulerClusterSpec) DeepCopyInto(out *DolphinschedulerClusterSpec) {
	*out = *in
	if in.Image != nil {
		in, out := &in.Image, &out.Image
		*out = new(ImageSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.ClusterConfig != nil {
		in, out := &in.ClusterConfig, &out.ClusterConfig
		*out = new(ClusterConfigSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.ClusterOperationSpec != nil {
		in, out := &in.ClusterOperationSpec, &out.ClusterOperationSpec
		*out = new(commonsv1alpha1.ClusterOperationSpec)
		**out = **in
	}
	if in.Master != nil {
		in, out := &in.Master, &out.Master
		*out = new(RoleSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.Worker != nil {
		in, out := &in.Worker, &out.Worker
		*out = new(RoleSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.Alerter != nil {
		in, out := &in.Alerter, &out.Alerter
		*out = new(RoleSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.Api != nil {
		in, out := &in.Api, &out.Api
		*out = new(RoleSpec)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DolphinschedulerClusterSpec.
func (in *DolphinschedulerClusterSpec) DeepCopy() *DolphinschedulerClusterSpec {
	if in == nil {
		return nil
	}
	out := new(DolphinschedulerClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ImageSpec) DeepCopyInto(out *ImageSpec) {
	*out = *in
	if in.PullPolicy != nil {
		in, out := &in.PullPolicy, &out.PullPolicy
		*out = new(v1.PullPolicy)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ImageSpec.
func (in *ImageSpec) DeepCopy() *ImageSpec {
	if in == nil {
		return nil
	}
	out := new(ImageSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OidcCredentialSecretSpec) DeepCopyInto(out *OidcCredentialSecretSpec) {
	*out = *in
	if in.ExtraScopes != nil {
		in, out := &in.ExtraScopes, &out.ExtraScopes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OidcCredentialSecretSpec.
func (in *OidcCredentialSecretSpec) DeepCopy() *OidcCredentialSecretSpec {
	if in == nil {
		return nil
	}
	out := new(OidcCredentialSecretSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RoleGroupSpec) DeepCopyInto(out *RoleGroupSpec) {
	*out = *in
	if in.Config != nil {
		in, out := &in.Config, &out.Config
		*out = new(ConfigSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.PodDisruptionBudget != nil {
		in, out := &in.PodDisruptionBudget, &out.PodDisruptionBudget
		*out = new(commonsv1alpha1.PodDisruptionBudgetSpec)
		**out = **in
	}
	if in.CommandOverrides != nil {
		in, out := &in.CommandOverrides, &out.CommandOverrides
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.ConfigOverrides != nil {
		in, out := &in.ConfigOverrides, &out.ConfigOverrides
		*out = new(ConfigOverridesSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.EnvOverrides != nil {
		in, out := &in.EnvOverrides, &out.EnvOverrides
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RoleGroupSpec.
func (in *RoleGroupSpec) DeepCopy() *RoleGroupSpec {
	if in == nil {
		return nil
	}
	out := new(RoleGroupSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RoleSpec) DeepCopyInto(out *RoleSpec) {
	*out = *in
	if in.Config != nil {
		in, out := &in.Config, &out.Config
		*out = new(ConfigSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.RoleGroups != nil {
		in, out := &in.RoleGroups, &out.RoleGroups
		*out = make(map[string]RoleGroupSpec, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
	if in.PodDisruptionBudget != nil {
		in, out := &in.PodDisruptionBudget, &out.PodDisruptionBudget
		*out = new(commonsv1alpha1.PodDisruptionBudgetSpec)
		**out = **in
	}
	if in.CommandOverrides != nil {
		in, out := &in.CommandOverrides, &out.CommandOverrides
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.ConfigOverrides != nil {
		in, out := &in.ConfigOverrides, &out.ConfigOverrides
		*out = new(ConfigOverridesSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.EnvOverrides != nil {
		in, out := &in.EnvOverrides, &out.EnvOverrides
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RoleSpec.
func (in *RoleSpec) DeepCopy() *RoleSpec {
	if in == nil {
		return nil
	}
	out := new(RoleSpec)
	in.DeepCopyInto(out)
	return out
}
