package controller

import (
	"context"
	"fmt"
	dolphinv1alpha1 "github.com/zncdata-labs/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/common"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/core"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/resource"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewJobInitScriptReconciler(
	scheme *runtime.Scheme,
	instance *dolphinv1alpha1.DolphinschedulerCluster,
	client client.Client,
	labels map[string]string,
	mergedCfg any,
) *JobInitScriptReconciler {
	return &JobInitScriptReconciler{
		GeneralResourceStyleReconciler: *core.NewGeneraResourceStyleReconciler(
			scheme,
			instance,
			client,
			"",
			labels,
			mergedCfg,
		),
	}
}

var _ core.ResourceBuilder = &JobInitScriptReconciler{}

type JobInitScriptReconciler struct {
	core.GeneralResourceStyleReconciler[*dolphinv1alpha1.DolphinschedulerCluster, any]
}

func (j *JobInitScriptReconciler) Build(ctx context.Context) (client.Object, error) {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      j.Instance.Name,
			Namespace: j.Instance.Namespace,
			Labels:    j.Labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						j.InitDbContainer(ctx),
					},
					InitContainers: []corev1.Container{
						j.waitDbContainer(ctx),
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}, nil
}

func (j *JobInitScriptReconciler) InitDbContainer(ctx context.Context) corev1.Container {
	return corev1.Container{
		Name:  string(ContainerDbInitJob),
		Image: dolphinv1alpha1.DbInitImage,
		Args: []string{
			"tools/bin/upgrade-schema.sh",
		},
		Env: j.InitDbContainerEnvs(ctx),
		EnvFrom: []corev1.EnvFromSource{
			{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.EnvsConfigMapName(j.Instance.GetName(), j.GroupName),
					},
				},
			},
		},
	}
}

func (j *JobInitScriptReconciler) InitDbContainerEnvs(ctx context.Context) []corev1.EnvVar {
	envs := []corev1.EnvVar{
		{
			Name:  "TZ",
			Value: "Asia/Shanghai",
		},
		{
			Name:  "SPRING_JACKSON_TIME_ZONE",
			Value: "Asia/Shanghai",
		},
		{
			Name:  "REGISTRY_TYPE",
			Value: "zookeeper",
		},
		{
			Name: "REGISTRY_ZOOKEEPER_CONNECT_STRING",
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: j.Instance.Spec.ClusterConfigSpec.ZookeeperDiscoveryZNode,
					},
					Key: core.ZookeeperDiscoveryKey,
				},
			},
		},
	}
	dbspec := j.Instance.Spec.ClusterConfigSpec.Database
	_, parmas := common.ExtractDataBaseReference(dbspec, ctx, j.Client, j.Instance.GetNamespace())
	envs = append(envs, common.MakeDataBaseEnvs(parmas)...)
	return envs
}

func (j *JobInitScriptReconciler) waitDbContainer(ctx context.Context) corev1.Container {
	dbspec := j.Instance.Spec.ClusterConfigSpec.Database
	_, params := common.ExtractDataBaseReference(dbspec, ctx, j.Client, j.Instance.GetNamespace())
	dbHost := params.Host
	return corev1.Container{
		Name:  string(ContainerWaitForDb),
		Image: "busybox:1.30.1",
		Command: []string{
			"sh",
			"-xc",
			fmt.Sprintf("for i in $(seq 1 180); do nc -z -w3 %s 5432 && exit 0 || sleep 5; done; exit 1", dbHost),
		},
	}
}

const (
	ContainerDbInitJob resource.ContainerComponent = "dolphinscheduler-db-init-job"
	ContainerWaitForDb resource.ContainerComponent = "wait-for-database"
)
