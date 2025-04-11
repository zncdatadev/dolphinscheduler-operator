package common

import (
	"context"
	"encoding/json"

	"emperror.dev/errors"
	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/builder"
	"github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/productlogging"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	"github.com/zncdatadev/operator-go/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func NewDeploymentReconciler(
	ctx context.Context,
	client *client.Client,
	stopped bool,
	image *util.Image,
	replicas *int32,
	roleGroupInfo *reconciler.RoleGroupInfo,
	overrides *commonsv1alpha1.OverridesSpec,
	roleGroupConfig *commonsv1alpha1.RoleGroupConfigSpec,
	containers []corev1.Container,
	volumes []corev1.Volume) reconciler.ResourceReconciler[builder.DeploymentBuilder] {
	deploymentBuilder := &DeploymentBuilder{
		Deployment: builder.NewDeployment(
			client,
			DeploymentName(roleGroupInfo),
			replicas,
			image,
			overrides,
			roleGroupConfig,
			func(o *builder.Options) {
				o.ClusterName = roleGroupInfo.GetClusterName()
				o.RoleGroupName = roleGroupInfo.GetGroupName()
				o.RoleName = roleGroupInfo.GetRoleName()
				o.Annotations = roleGroupInfo.GetAnnotations()
				o.Labels = roleGroupInfo.GetLabels()
			},
		),
		WorkloadBuilder: NewWorkloadBuilder(roleGroupConfig, roleGroupInfo, containers),
	}
	if len(volumes) > 0 {
		deploymentBuilder.WithVolumes(volumes)
	}
	return reconciler.NewDeployment(client, deploymentBuilder, stopped)
}
func NewStatefulSetReconciler(
	ctx context.Context,
	client *client.Client,
	stopped bool,
	image *util.Image,
	replicas *int32,
	roleGroupInfo *reconciler.RoleGroupInfo,
	overrides *commonsv1alpha1.OverridesSpec,
	roleGroupConfig *commonsv1alpha1.RoleGroupConfigSpec,
	containers []corev1.Container,
	pvcName string,
	storageSize *resource.Quantity) reconciler.ResourceReconciler[builder.StatefulSetBuilder] {
	b := NewWorkloadBuilder(roleGroupConfig, roleGroupInfo, containers)
	if pvcName != "" {
		b.WithPvcTemplates(pvcName, *storageSize)
	}
	stsBuilder := &StatefulSetBuilder{
		StatefulSet: builder.NewStatefulSetBuilder(
			client,
			StatefulsetName(roleGroupInfo),
			replicas,
			image,
			overrides,
			roleGroupConfig,
			func(o *builder.Options) {
				o.ClusterName = roleGroupInfo.GetClusterName()
				o.RoleGroupName = roleGroupInfo.GetGroupName()
				o.RoleName = roleGroupInfo.GetRoleName()
				o.Annotations = roleGroupInfo.GetAnnotations()
				o.Labels = roleGroupInfo.GetLabels()
			},
		),
		WorkloadBuilder: b,
	}
	return reconciler.NewStatefulSet(client, stsBuilder, stopped)
}

func NewWorkloadBuilder(
	roleGroupConfig *commonsv1alpha1.RoleGroupConfigSpec,
	rolegroupInfo *reconciler.RoleGroupInfo,
	containers []corev1.Container) *WorkloadBuilder {
	builder := &WorkloadBuilder{
		RoleGroupInf:    rolegroupInfo,
		Containers:      containers,
		RoleGroupConfig: roleGroupConfig,
	}
	builder.volumes = builder.commonVolumes()
	return builder
}

var _ builder.DeploymentBuilder = &DeploymentBuilder{}
var _ builder.StatefulSetBuilder = &StatefulSetBuilder{}

type DeploymentBuilder struct {
	*builder.Deployment
	*WorkloadBuilder
}

func (b *DeploymentBuilder) Build(ctx context.Context) (ctrlclient.Object, error) {
	if affinity, err := b.ExtractAffinity(); err != nil {
		return nil, err
	} else if affinity != nil {
		b.SetAffinity(affinity)
	}
	b.AddContainers(b.Containers)
	b.AddVolumes(b.volumes)
	// b.SetSecurityContext(1001, 1001, false)

	obj, err := b.GetObject()
	if err != nil {
		return nil, err
	}
	b.VectorDecorator(obj, b.GetImage()) // vector

	obj.Spec.Template.Spec.ServiceAccountName = b.serviceAccount()
	return obj, nil
}

// statefulset builder
type StatefulSetBuilder struct {
	*builder.StatefulSet
	*WorkloadBuilder
}

func (b *StatefulSetBuilder) Build(ctx context.Context) (ctrlclient.Object, error) {
	if affinity, err := b.ExtractAffinity(); err != nil {
		return nil, err
	} else if affinity != nil {
		b.SetAffinity(affinity)
	}
	b.AddContainers(b.Containers)
	b.AddVolumes(b.volumes)
	// b.SetSecurityContext(1001, 1001, false)

	obj, err := b.GetObject()
	if err != nil {
		return nil, err
	}

	obj.Spec.Template.Spec.ServiceAccountName = b.serviceAccount()
	if len(b.pvcs) > 0 {
		obj.Spec.VolumeClaimTemplates = b.pvcs
	}
	b.VectorDecorator(obj, b.GetImage()) // vector
	return obj, nil
}

// workload builder
type WorkloadBuilder struct {
	RoleGroupInf    *reconciler.RoleGroupInfo
	Containers      []corev1.Container
	RoleGroupConfig *commonsv1alpha1.RoleGroupConfigSpec

	pvcs    []corev1.PersistentVolumeClaim
	volumes []corev1.Volume
}

// decoraate vector
func (w *WorkloadBuilder) VectorDecorator(workloadObject ctrlclient.Object, image *util.Image) {
	if IsVectorEnable(w.RoleGroupConfig.Logging) {
		// ExtendWorkloadByVector(image, workloadObject, RoleGroupConfigMapName(w.RoleGroupInf))
		vectorFactory := GetVecctorFactory(image)

		switch obj := workloadObject.(type) {
		case *appsv1.Deployment:
			obj.Spec.Template.Spec.Containers = append(obj.Spec.Template.Spec.Containers, *vectorFactory.GetContainer())
			obj.Spec.Template.Spec.Volumes = append(obj.Spec.Template.Spec.Volumes, vectorFactory.GetVolumes()...)
		case *appsv1.StatefulSet:
			obj.Spec.Template.Spec.Containers = append(obj.Spec.Template.Spec.Containers, *vectorFactory.GetContainer())
			obj.Spec.Template.Spec.Volumes = append(obj.Spec.Template.Spec.Volumes, vectorFactory.GetVolumes()...)
		default:
			// Handle other types of workload objects if needed
			logger.Error(errors.New("unsupported workload type"), "unsupported workload type", "type", obj.GetObjectKind().GroupVersionKind())
			return
		}
	}
}

// set Affinity
func (w *WorkloadBuilder) ExtractAffinity() (*corev1.Affinity, error) {
	if w.RoleGroupConfig != nil && w.RoleGroupConfig.Affinity != nil {
		affinity, err := convertRawExtension[corev1.Affinity](w.RoleGroupConfig.Affinity)
		if err != nil {
			return nil, err
		}
		return affinity, nil
	}
	return nil, nil
}

// with volumes
func (w *WorkloadBuilder) WithVolumes(volumes []corev1.Volume) {
	w.volumes = append(w.volumes, volumes...)
}

func (w *WorkloadBuilder) commonVolumes() []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: dolphinv1alpha1.LoggingVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: func() *resource.Quantity {
						q := resource.MustParse(dolphinv1alpha1.MaxLogFileSize)
						size := productlogging.CalculateLogVolumeSizeLimit([]resource.Quantity{q})
						return &size
					}(),
				},
			},
		},
		{
			Name: dolphinv1alpha1.ConfigVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: RoleGroupConfigMapName(w.RoleGroupInf),
					},
				},
			},
		},
		{
			Name: dolphinv1alpha1.LogbackVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: RoleGroupConfigMapName(w.RoleGroupInf),
					},
					Items: []corev1.KeyToPath{
						{
							Key:  dolphinv1alpha1.LogbackPropertiesFileName,
							Path: dolphinv1alpha1.LogbackPropertiesFileName,
						},
					},
				},
			},
		},
	}
	return volumes
}

// with service Account
func (w *WorkloadBuilder) serviceAccount() string {
	return ServiceAccountName(dolphinv1alpha1.DefaultProductName)
}

// with pvc templates
func (w *WorkloadBuilder) WithPvcTemplates(pvcName string, storageSize resource.Quantity) *WorkloadBuilder {
	// assert w is statefulset
	w.pvcs = []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: pvcName,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				VolumeMode:  func() *corev1.PersistentVolumeMode { v := corev1.PersistentVolumeFilesystem; return &v }(),
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: storageSize,
					},
				},
			},
		},
	}
	return w
}

// get pvc templates
func (w *WorkloadBuilder) GetPvcTemplates() []corev1.PersistentVolumeClaim {
	return w.pvcs
}

func convertRawExtension[T any](raw *runtime.RawExtension) (*T, error) {
	var obj T
	if raw == nil || raw.Raw == nil {
		return &obj, nil
	}

	if err := json.Unmarshal(raw.Raw, &obj); err != nil {
		return &obj, err
	}
	return &obj, nil
}
