package resource

import (
	"context"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/core"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"reflect"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

func NewGenericStatefulSetReconciler[T client.Object, G any](
	scheme *runtime.Scheme,
	instance T,
	client client.Client,
	groupName string,
	mergedLabels map[string]string,
	mergedCfg G,
	replicas int32,
	requirements WorkloadBuilderRequirements,
) *GenericStatefulSetReconciler[T, G] {
	return &GenericStatefulSetReconciler[T, G]{
		WorkloadStyleReconciler: *core.NewWorkloadStyleReconciler[T, G](
			scheme,
			instance,
			client,
			groupName,
			mergedLabels,
			mergedCfg,
			replicas,
			requirements,
		),
		requirements: requirements,
	}
}

func NewGenericDeploymentReconciler[T client.Object, G any](
	scheme *runtime.Scheme,
	instance T,
	client client.Client,
	groupName string,
	mergedLabels map[string]string,
	mergedCfg G,
	replicas int32,
	requirements WorkloadBuilderRequirements,
) *GenericDeploymentReconciler[T, G] {
	return &GenericDeploymentReconciler[T, G]{
		WorkloadStyleReconciler: *core.NewWorkloadStyleReconciler[T, G](
			scheme,
			instance,
			client,
			groupName,
			mergedLabels,
			mergedCfg,
			replicas,
			requirements,
		),
		requirements: requirements,
	}
}

// statefulSet builder

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

// deployment builder

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

func NewGenericWorkloadRequirements(
	mainContainerName string,
	containers *[]corev1.Container,
	cmdOverride []string,
	envOverrides map[string]string,
	instanceCondition *[]metav1.Condition) *GenericWorkloadOverrideHandler {
	return &GenericWorkloadOverrideHandler{
		MainContainerName: mainContainerName,
		Containers:        containers,
		CmdOverride:       cmdOverride,
		EnvOverrides:      envOverrides,
		InstanceCondition: instanceCondition,
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

func (s *StatefulSetBuilder) Build(_ context.Context) (client.Object, error) {
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
	return statefulSet, nil
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

func (d *DeploymentBuilder) Build(_ context.Context) (client.Object, error) {
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
	return deployment, nil
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

// WorkloadBuilderRequirements workload buidler requirements
// all workload builder requirements must implement this interface
// it include resource builder and workload resource requirements interface
// ResourceBuilder: core.ResourceBuilder, should build the single resource, such as StatefulSetBuilder, DeploymentBuilder
// WorkloadResourceRequirements: core.WorkloadResourceRequirements, should implement the workload resource requirements,
// such as command, envs, logging override, and get instance condition, usually we can create it by NewGenericWorkloadRequirements
type WorkloadBuilderRequirements interface {
	core.ResourceBuilder
	core.WorkloadResourceRequirements
}

type GenericStatefulSetReconciler[T client.Object, G any] struct {
	core.WorkloadStyleReconciler[T, G]
	requirements WorkloadBuilderRequirements
}

func (g *GenericStatefulSetReconciler[T, G]) Build(ctx context.Context) (client.Object, error) {
	return g.requirements.Build(ctx)
}

type GenericDeploymentReconciler[T client.Object, G any] struct {
	core.WorkloadStyleReconciler[T, G]
	requirements WorkloadBuilderRequirements
}

func (g *GenericDeploymentReconciler[T, G]) Build(ctx context.Context) (client.Object, error) {
	return g.requirements.Build(ctx)
}

// WorkloadResourceRequirements workload reconciler requirements
// do works below:
// 1. command and env override can support
// 2. logging override can support
// 3. get instance condition

var _ core.WorkloadResourceRequirements = &GenericWorkloadOverrideHandler{}

type GenericWorkloadOverrideHandler struct {
	MainContainerName string
	Containers        *[]corev1.Container
	CmdOverride       []string
	EnvOverrides      map[string]string
	InstanceCondition *[]metav1.Condition
	*LoggingOverrideHandler
}

func (s *GenericWorkloadOverrideHandler) CommandOverride(_ client.Object) {
	if s.CmdOverride != nil {
		containers := *s.Containers
		for i := range containers {
			if containers[i].Name == s.MainContainerName {
				containers[i].Command = s.CmdOverride
				break
			}
		}
	}
}

func (s *GenericWorkloadOverrideHandler) EnvOverride(_ client.Object) {
	if len(s.EnvOverrides) > 0 {
		containers := *s.Containers
		for i := range containers {
			if containers[i].Name == s.MainContainerName {
				envVars := containers[i].Env
				OverrideEnvVars(&envVars, s.EnvOverrides)
				break
			}
		}
	}
}

func (s *GenericWorkloadOverrideHandler) LogOverride(obj client.Object) {
	if s.LoggingOverrideHandler != nil {
		containers := *s.Containers
		for i := range containers {
			if containers[i].Name == s.MainContainerName {
				s.LoggingOverrideHandler.LogOverride(obj, &containers[i])
				break
			}
		}
	}
}

func (s *GenericWorkloadOverrideHandler) GetConditions() *[]metav1.Condition {
	return s.InstanceCondition
}

func OverrideEnvVars(origin *[]corev1.EnvVar, override map[string]string) {
	var originVars = make(map[string]int)
	for i, env := range *origin {
		originVars[env.Name] = i
	}

	for k, v := range override {
		// if env Name is in override, then override it
		if idx, ok := originVars[k]; ok {
			(*origin)[idx].Value = v
		} else {
			// if override's key is new, then append it
			*origin = append(*origin, corev1.EnvVar{
				Name:  k,
				Value: v,
			})
		}
	}
}

// LoggingOverrideHandler logging override handler
// append log volumes to exist workload resource,
// append log volume mounts to exist main container in exist workload resource
type LoggingOverrideHandler struct {
	LoggingVolumeName string
	LoggingConfigName string
	LoggingPath       string
	LoggingFile       string

	WorkloadVolumes      *[]corev1.Volume
	WorkloadVolumeMounts *[]corev1.VolumeMount
	// other fields
}

func NewLoggingOverrideHandler(logVolumeName string, logConfigName string, logPath string,
	logFile string) *LoggingOverrideHandler {
	return &LoggingOverrideHandler{
		LoggingVolumeName: logVolumeName,
		LoggingConfigName: logConfigName,
		LoggingPath:       logPath,
		LoggingFile:       logFile,
	}
}

func (s *LoggingOverrideHandler) LogOverride(obj client.Object, mainContainer *corev1.Container) {
	// main container volume mounts
	s.WorkloadVolumeMounts = &mainContainer.VolumeMounts

	// workload volumes
	switch obj := obj.(type) {
	case *appsv1.StatefulSet:
		s.WorkloadVolumes = &obj.Spec.Template.Spec.Volumes
	case *appsv1.Deployment:
		s.WorkloadVolumes = &obj.Spec.Template.Spec.Volumes
	default:
		panic("obj is not StatefulSet or Deployment, current is " + reflect.TypeOf(obj).String())
	}

	// update volume and volume mounts
	s.AppendVolumes()
	s.AppendVolumeMounts()
}

// AppendVolumes append log volume to exist volumes
func (s *LoggingOverrideHandler) AppendVolumes() *[]corev1.Volume {
	*s.WorkloadVolumes = append(*s.WorkloadVolumes, corev1.Volume{
		Name: s.LoggingVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: s.LoggingConfigName,
				},
			},
		},
	})
	return s.WorkloadVolumes
}

// AppendVolumeMounts append log volume mount to exist volume mounts
func (s *LoggingOverrideHandler) AppendVolumeMounts() *[]corev1.VolumeMount {
	*s.WorkloadVolumeMounts = append(*s.WorkloadVolumeMounts, corev1.VolumeMount{
		Name:      s.LoggingVolumeName,
		MountPath: s.LoggingPath,
		SubPath:   s.LoggingFile,
	})
	return s.WorkloadVolumeMounts
}
