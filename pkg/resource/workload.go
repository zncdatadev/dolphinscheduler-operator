package resource

import (
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/core"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// statefulSet

func NewStatefulSetBuilder(name string, nameSpace string, labels map[string]string, replicas int32,
	serviceName string, containers []corev1.Container) *StatefulSetBuilder {
	return &StatefulSetBuilder{
		Name:        name,
		NameSpace:   nameSpace,
		Labels:      labels,
		Replicas:    replicas,
		ServiceName: serviceName,
		Containers:  containers,
	}
}

// deployment

func NewDeploymentBuilder(name string, nameSpace string, labels map[string]string, replicas int32,
	containers []corev1.Container) *DeploymentBuilder {
	return &DeploymentBuilder{
		Name:       name,
		NameSpace:  nameSpace,
		Labels:     labels,
		Replicas:   replicas,
		Containers: containers,
	}
}

type WorkloadResourceType interface {
	core.ResourceBuilder
	core.ConditionsGetter
	core.WorkloadOverride
}

type VolumeSourceType string

const (
	ConfigMap       VolumeSourceType = "configmap"
	Secret          VolumeSourceType = "secret"
	EmptyDir        VolumeSourceType = "emptyDir"
	EphemeralSecret VolumeSourceType = "ephemeralSecret"
)

var VolumeTypeHandlers = map[VolumeSourceType]func(string, *VolumeSourceParams) corev1.VolumeSource{
	EmptyDir: func(name string, params *VolumeSourceParams) corev1.VolumeSource {
		return corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				SizeLimit: func() *resource.Quantity {
					if params != nil && params.EmptyVolumeLimit != "" {
						q := resource.MustParse(params.EmptyVolumeLimit)
						return &q
					}
					return nil
				}(),
			},
		}
	},
	ConfigMap: func(name string, params *VolumeSourceParams) corev1.VolumeSource {
		param := params.ConfigMap
		source := corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: param.Name,
				},
			},
		}
		if len(param.KeyPath) > 0 {
			source.ConfigMap.Items = params.ConfigMap.KeyPath
		}
		return source
	},
	Secret: func(name string, params *VolumeSourceParams) corev1.VolumeSource {
		return corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: params.SecretName,
			},
		}
	},
	EphemeralSecret: func(name string, params *VolumeSourceParams) corev1.VolumeSource {
		param := params.EphemeralSecret
		return corev1.VolumeSource{
			Ephemeral: &corev1.EphemeralVolumeSource{
				VolumeClaimTemplate: CreateListenPvcTemplate(param.Annotations, param.StorageClass, param.AccessModes,
					param.StorageSize),
			},
		}
	},
}

type PvcSpec struct {
	StorageClass *string
	AccessModes  []corev1.PersistentVolumeAccessMode
	StorageSize  string
}

type EphemeralSecretSpec struct {
	PvcSpec
	Annotations map[string]string
}

type ConfigMapSpec struct {
	Name    string
	KeyPath []corev1.KeyToPath
}

type VolumeSourceParams struct {
	EmptyVolumeLimit string
	ConfigMap        ConfigMapSpec
	SecretName       string
	EphemeralSecret  *EphemeralSecretSpec
}

type VolumeSpec struct {
	Name       string
	SourceType VolumeSourceType
	Params     *VolumeSourceParams
}

func CreateListenPvcTemplate(annotations map[string]string, storageClass *string,
	accessMode []corev1.PersistentVolumeAccessMode, storageSize string) *corev1.PersistentVolumeClaimTemplate {
	mode := corev1.PersistentVolumeFilesystem
	pvcTemplate := &corev1.PersistentVolumeClaimTemplate{
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeMode:       &mode,
			StorageClassName: storageClass,
			AccessModes:      accessMode,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(storageSize),
				},
			},
		},
	}
	if annotations != nil {
		pvcTemplate.Annotations = annotations
	}
	return pvcTemplate
}

type VolumeClaimTemplateSpec struct {
	Name string
	PvcSpec
}

type StatefulSetBuilder struct {
	Name               string
	NameSpace          string
	Labels             map[string]string
	Replicas           int32
	ServiceName        string
	ServiceAccountName string
	Containers         []corev1.Container
	InitContainers     []corev1.Container
	Volumes            []VolumeSpec
	PvcTemplates       []VolumeClaimTemplateSpec
}

func (s *StatefulSetBuilder) SetServiceAccountName(saName string) *StatefulSetBuilder {
	s.ServiceAccountName = saName
	return s
}

func (s *StatefulSetBuilder) SetVolumes(volumes []VolumeSpec) *StatefulSetBuilder {
	s.Volumes = volumes
	return s
}

func (s *StatefulSetBuilder) SetInitContainers(containers []corev1.Container) *StatefulSetBuilder {
	s.InitContainers = containers
	return s
}

func (s *StatefulSetBuilder) SetPvcTemplates(templates []VolumeClaimTemplateSpec) *StatefulSetBuilder {
	s.PvcTemplates = templates
	return s
}

func (s *StatefulSetBuilder) Build() *appsv1.StatefulSet {
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: s.NameSpace,
			Labels:    s.Labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &s.Replicas,
			ServiceName: s.ServiceName,
			Selector: &metav1.LabelSelector{
				MatchLabels: s.Labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: s.Labels,
				},
				Spec: corev1.PodSpec{
					Containers: s.Containers,
				},
			},
		},
	}

	if s.ServiceAccountName != "" {
		statefulSet.Spec.Template.Spec.ServiceAccountName = s.ServiceAccountName
	}

	if len(s.InitContainers) > 0 {
		statefulSet.Spec.Template.Spec.InitContainers = s.InitContainers
	}

	if len(s.Volumes) > 0 {
		statefulSet.Spec.Template.Spec.Volumes = s.createVolumes()
	}

	if len(s.PvcTemplates) > 0 {
		statefulSet.Spec.VolumeClaimTemplates = s.createPvcTemplates()
	}
	return statefulSet
}

// create statefulSet volumes
func (s *StatefulSetBuilder) createVolumes() []corev1.Volume {
	volumes := make([]corev1.Volume, 0)
	for _, v := range s.Volumes {
		volumeHandler := VolumeTypeHandlers[v.SourceType]
		volumes = append(volumes, corev1.Volume{
			Name:         v.Name,
			VolumeSource: volumeHandler(v.Name, v.Params),
		})
	}
	return volumes
}

// create statefulSet pvcTemplates
func (s *StatefulSetBuilder) createPvcTemplates() []corev1.PersistentVolumeClaim {
	pvcTemplates := make([]corev1.PersistentVolumeClaim, 0)
	mode := corev1.PersistentVolumeFilesystem
	for _, v := range s.PvcTemplates {
		pvcTemplates = append(pvcTemplates, corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v.Name,
				Namespace: s.NameSpace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				VolumeMode:       &mode,
				AccessModes:      v.AccessModes,
				StorageClassName: v.StorageClass,
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(v.StorageSize)},
				},
			},
		})
	}
	return pvcTemplates
}

type DeploymentBuilder struct {
	Name               string
	NameSpace          string
	Labels             map[string]string
	Replicas           int32
	ServiceAccountName string
	Containers         []corev1.Container
	InitContainers     []corev1.Container
	Volumes            []VolumeSpec
}

func (d *DeploymentBuilder) SetServiceAccountName(saName string) *DeploymentBuilder {
	d.ServiceAccountName = saName
	return d
}

func (d *DeploymentBuilder) SetVolumes(volumes []VolumeSpec) *DeploymentBuilder {
	d.Volumes = volumes
	return d
}

func (d *DeploymentBuilder) SetInitContainers(containers []corev1.Container) *DeploymentBuilder {
	d.InitContainers = containers
	return d
}

func (d *DeploymentBuilder) Build() *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.Name,
			Namespace: d.NameSpace,
			Labels:    d.Labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &d.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: d.Labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: d.Labels,
				},
				Spec: corev1.PodSpec{
					Containers: d.Containers,
				},
			},
		},
	}
	if d.ServiceAccountName != "" {
		deployment.Spec.Template.Spec.ServiceAccountName = d.ServiceAccountName
	}
	if len(d.InitContainers) > 0 {
		deployment.Spec.Template.Spec.InitContainers = d.InitContainers
	}
	if len(d.Volumes) > 0 {
		deployment.Spec.Template.Spec.Volumes = d.createVolumes()
	}
	return deployment
}

// create deployment volumes
func (d *DeploymentBuilder) createVolumes() []corev1.Volume {
	volumes := make([]corev1.Volume, 0)
	for _, v := range d.Volumes {
		volumeHandler := VolumeTypeHandlers[v.SourceType]
		volumes = append(volumes, corev1.Volume{
			Name:         v.Name,
			VolumeSource: volumeHandler(v.Name, v.Params),
		})
	}
	return volumes
}
