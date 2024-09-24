package common

import (
	"context"

	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/builder"
	"github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	"github.com/zncdatadev/operator-go/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func NewDeploymentReconciler(
	ctx context.Context,
	mergedCfg *dolphinv1alpha1.RoleGroupSpec,
	client *client.Client,
	stopped bool,
	image *util.Image,
	options builder.WorkloadOptions,
	roleGroupInfo *reconciler.RoleGroupInfo,
	containers []corev1.Container) reconciler.ResourceReconciler[builder.DeploymentBuilder] {
	deploymentBuilder := &DeploymentBuilder{
		Deployment:      builder.NewDeployment(client, DeploymentName(roleGroupInfo), &mergedCfg.Replicas, image, options),
		WorkloadBuilder: NewWorkloadBuilder(mergedCfg, roleGroupInfo, containers),
	}
	return reconciler.NewDeployment(client, DeploymentName(roleGroupInfo), deploymentBuilder, stopped)
}
func NewStatefulSetReconciler(
	ctx context.Context,
	mergedCfg *dolphinv1alpha1.RoleGroupSpec,
	client *client.Client,
	stopped bool,
	image *util.Image,
	options builder.WorkloadOptions,
	roleGroupInfo *reconciler.RoleGroupInfo,
	containers []corev1.Container, pvcName string, storageSize *resource.Quantity) reconciler.ResourceReconciler[builder.StatefulSetBuilder] {
	b := NewWorkloadBuilder(mergedCfg, roleGroupInfo, containers)
	if pvcName != "" {
		b.WithPvcTemplates(pvcName, *storageSize)
	}
	stsBuilder := &StatefulSetBuilder{
		StatefulSet:     builder.NewStatefulSetBuilder(client, StatefulsetName(roleGroupInfo), &mergedCfg.Replicas, image, options),
		WorkloadBuilder: b,
	}
	return reconciler.NewStatefulSet(client, StatefulsetName(roleGroupInfo), stsBuilder, stopped)
}

func NewWorkloadBuilder(
	mergedCfg *dolphinv1alpha1.RoleGroupSpec,
	rolegroupInfo *reconciler.RoleGroupInfo,
	containers []corev1.Container) *WorkloadBuilder {
	return &WorkloadBuilder{
		RoleGroupInf: rolegroupInfo,
		Containers:   containers,
		MergedCfg:    mergedCfg,
	}
}

var _ builder.DeploymentBuilder = &DeploymentBuilder{}
var _ builder.StatefulSetBuilder = &StatefulSetBuilder{}

type DeploymentBuilder struct {
	*builder.Deployment
	*WorkloadBuilder
}

func (b *DeploymentBuilder) Build(ctx context.Context) (ctrlclient.Object, error) {
	b.SetAffinity(b.MergedCfg.Config.Affinity)
	b.AddContainers(b.Containers)
	b.AddVolumes(b.volumes())
	// b.SetSecurityContext(1001, 1001, false)

	obj, err := b.GetObject()
	if err != nil {
		return nil, err
	}

	obj.Spec.Template.Spec.ServiceAccountName = b.serviceAccount()
	return obj, nil
}

// statefulset builder
type StatefulSetBuilder struct {
	*builder.StatefulSet
	*WorkloadBuilder
}

func (b *StatefulSetBuilder) Build(ctx context.Context) (ctrlclient.Object, error) {
	b.SetAffinity(b.MergedCfg.Config.Affinity)
	b.AddContainers(b.Containers)
	b.AddVolumes(b.volumes())
	// b.SetSecurityContext(1001, 1001, false)

	obj, err := b.GetObject()
	if err != nil {
		return nil, err
	}

	obj.Spec.Template.Spec.ServiceAccountName = b.serviceAccount()
	if len(b.pvcs) > 0 {
		obj.Spec.VolumeClaimTemplates = b.pvcs
	}
	return obj, nil
}

// workload builder
type WorkloadBuilder struct {
	RoleGroupInf *reconciler.RoleGroupInfo
	Containers   []corev1.Container
	MergedCfg    *dolphinv1alpha1.RoleGroupSpec

	pvcs []corev1.PersistentVolumeClaim
}

// with volumes
func (w *WorkloadBuilder) volumes() []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: dolphinv1alpha1.CommonPropertiesVolumeName,
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
	//assert w is statefulset
	w.pvcs = []corev1.PersistentVolumeClaim{
		corev1.PersistentVolumeClaim{
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
