package worker

import (
	dolphinv1alpha1 "github.com/zncdata-labs/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/common"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/core"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/resource"
	corev1 "k8s.io/api/core/v1"
	"strconv"
)

func NewWorkerContainerBuilder(
	image string,
	imagePullPolicy corev1.PullPolicy,
	zookeeperDiscoveryZNode string,
	resourceSpec *dolphinv1alpha1.ResourcesSpec,
	envConfigName string,
	configConfigMapName string,
	dbSpec *dolphinv1alpha1.DatabaseSpec,
) *ContainerBuilder {
	if dbSpec == nil {
		panic("dbSpec is nil")
	}
	return &ContainerBuilder{
		ContainerBuilder:        *resource.NewContainerBuilder(image, imagePullPolicy),
		zookeeperDiscoveryZNode: zookeeperDiscoveryZNode,
		resourceSpec:            resourceSpec,
		envConfigName:           envConfigName,
		configConfigMapName:     configConfigMapName,
		dbSpec:                  dbSpec,
	}
}

type ContainerWorkerBuilderType interface {
	resource.ContainerName
	resource.ContainerEnv
	resource.ContainerEnvFrom
	resource.VolumeMount
	resource.LivenessProbe
	resource.ReadinessProbe
	resource.ContainerPorts
}

var _ ContainerWorkerBuilderType = &ContainerBuilder{}

type ContainerBuilder struct {
	resource.ContainerBuilder
	zookeeperDiscoveryZNode string
	resourceSpec            *dolphinv1alpha1.ResourcesSpec
	envConfigName           string
	configConfigMapName     string
	dbSpec                  *dolphinv1alpha1.DatabaseSpec
}

func (c *ContainerBuilder) ContainerPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			ContainerPort: dolphinv1alpha1.WorkerPort,
			Name:          dolphinv1alpha1.WorkerPortName,
			Protocol:      corev1.ProtocolTCP,
		},
		{
			ContainerPort: dolphinv1alpha1.WorkerActualPort,
			Name:          dolphinv1alpha1.WorkerActualPortName,
			Protocol:      corev1.ProtocolTCP,
		},
	}
}

func (c *ContainerBuilder) ContainerEnvFromSource() []corev1.EnvFromSource {
	return []corev1.EnvFromSource{
		{
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: c.envConfigName,
				},
			},
		},
	}
}

func (c *ContainerBuilder) ContainerEnv() []corev1.EnvVar {
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
						Name: c.zookeeperDiscoveryZNode,
					},
					Key: core.ZookeeperDiscoveryKey,
				},
			},
		},
		{
			Name:  "DEFAULT_TENANT_ENABLED",
			Value: "false",
		},
		{
			Name:  "WORKER_EXEC_THREADS",
			Value: "100",
		},
		{
			Name:  "WORKER_HOST_WEIGHT",
			Value: "100",
		},
		{
			Name:  "WORKER_MAX_HEARTBEAT_INTERVAL",
			Value: "10s",
		},
		{
			Name:  "WORKER_SERVER_LOAD_PROTECTION_ENABLED",
			Value: "false",
		},
		{
			Name:  "WORKER_SERVER_LOAD_PROTECTION_MAX_DISK_USAGE_PERCENTAGE_THRESHOLDS",
			Value: "0.7",
		},
		{
			Name:  "WORKER_SERVER_LOAD_PROTECTION_MAX_JVM_CPU_USAGE_PERCENTAGE_THRESHOLDS",
			Value: "0.7",
		},
		{
			Name:  "WORKER_SERVER_LOAD_PROTECTION_MAX_SYSTEM_CPU_USAGE_PERCENTAGE_THRESHOLDS",
			Value: "0.7",
		},
		{
			Name:  "WORKER_SERVER_LOAD_PROTECTION_MAX_SYSTEM_MEMORY_USAGE_PERCENTAGE_THRESHOLDS",
			Value: "0.7",
		},
		{
			Name:  "WORKER_TENANT_CONFIG_AUTO_CREATE_TENANT_ENABLED",
			Value: "true",
		},
		{
			Name:  "WORKER_TENANT_CONFIG_DISTRIBUTED_TENANT",
			Value: "false",
		},
	}
	envs = append(envs, common.MakeDataBaseEnvs(c.dbSpec)...)
	return envs
}

func (c *ContainerBuilder) VolumeMount() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      workerDataVolumeName(),
			MountPath: "/tmp/dolphinscheduler",
		},
		{
			Name:      configVolumeName(),
			MountPath: "/opt/dolphinscheduler/conf/common.properties",
			SubPath:   dolphinv1alpha1.DolphinCommonPropertiesName,
		},
	}
}

func (c *ContainerBuilder) LivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{
					"curl",
					"-s",
					"http://localhost:" + strconv.Itoa(dolphinv1alpha1.WorkerActualPort) + "/actuator/health/liveness",
				},
			},
		},
		InitialDelaySeconds: 30,
		SuccessThreshold:    1,
		FailureThreshold:    3,
		PeriodSeconds:       30,
		TimeoutSeconds:      5,
	}
}

func (c *ContainerBuilder) ReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{
					"curl",
					"-s",
					"http://localhost:" + strconv.Itoa(dolphinv1alpha1.WorkerActualPort) + "/actuator/health/readiness",
				},
			},
		},
		InitialDelaySeconds: 30,
		SuccessThreshold:    1,
		FailureThreshold:    3,
		PeriodSeconds:       30,
		TimeoutSeconds:      5,
	}
}

func (c *ContainerBuilder) ContainerName() string {
	return string(ContainerWorker)
}

const ContainerWorker resource.ContainerComponent = resource.ContainerComponent(core.Worker)
