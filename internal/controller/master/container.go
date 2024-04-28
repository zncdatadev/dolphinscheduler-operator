package master

import (
	dolphinv1alpha1 "github.com/zncdata-labs/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/common"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/core"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/resource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func NewMasterContainerBuilder(
	image string,
	imagePullPolicy corev1.PullPolicy,
	zookeeperDiscoveryZNode string,
	resourceSpec *dolphinv1alpha1.ResourcesSpec,
	envConfigName string,
	configConfigMapName string,
	dbParams *resource.DatabaseParams,
) *ContainerBuilder {
	if dbParams == nil {
		panic("database connection info is nil")
	}
	return &ContainerBuilder{
		ContainerBuilder:        *resource.NewContainerBuilder(image, imagePullPolicy),
		zookeeperDiscoveryZNode: zookeeperDiscoveryZNode,
		resourceSpec:            resourceSpec,
		envConfigName:           envConfigName,
		configConfigMapName:     configConfigMapName,
		dbParams:                dbParams,
	}
}

type ContainerMasterBuilderType interface {
	resource.ContainerName
	resource.ContainerEnv
	resource.ContainerEnvFrom
	resource.VolumeMount
	resource.LivenessProbe
	resource.ReadinessProbe
	resource.ContainerPorts
}

var _ ContainerMasterBuilderType = &ContainerBuilder{}

type ContainerBuilder struct {
	resource.ContainerBuilder
	zookeeperDiscoveryZNode string
	resourceSpec            *dolphinv1alpha1.ResourcesSpec
	envConfigName           string
	configConfigMapName     string
	dbParams                *resource.DatabaseParams
}

func (c *ContainerBuilder) ContainerPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			ContainerPort: dolphinv1alpha1.MasterPort,
			Name:          dolphinv1alpha1.MasterPortName,
			Protocol:      corev1.ProtocolTCP,
		},
		{
			ContainerPort: dolphinv1alpha1.MasterActualPort,
			Name:          dolphinv1alpha1.MasterActualPortName,
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
			Name:  "JAVA_OPTS",
			Value: "-Xms1g -Xmx1g -Xmn512m",
		},
		{
			Name:  "MASTER_DISPATCH_TASK_NUM",
			Value: "3",
		},
		{
			Name:  "MASTER_EXEC_TASK_NUM",
			Value: "20",
		},
		{
			Name:  "MASTER_EXEC_THREADS",
			Value: "100",
		},
		{
			Name:  "MASTER_FAILOVER_INTERVAL",
			Value: "10m",
		},
		{
			Name:  "MASTER_HEARTBEAT_ERROR_THRESHOLD",
			Value: "5",
		},
		{
			Name:  "MASTER_HOST_SELECTOR",
			Value: "LowerWeight",
		},
		{
			Name:  "MASTER_KILL_APPLICATION_WHEN_HANDLE_FAILOVER",
			Value: "true",
		},
		{
			Name:  "MASTER_MAX_HEARTBEAT_INTERVAL",
			Value: "10s",
		},
		{
			Name:  "MASTER_SERVER_LOAD_PROTECTION_ENABLED",
			Value: "false",
		},
		{
			Name:  "MASTER_SERVER_LOAD_PROTECTION_MAX_DISK_USAGE_PERCENTAGE_THRESHOLDS",
			Value: "0.7",
		},
		{
			Name:  "MASTER_SERVER_LOAD_PROTECTION_MAX_JVM_CPU_USAGE_PERCENTAGE_THRESHOLDS",
			Value: "0.7",
		},
		{
			Name:  "MASTER_SERVER_LOAD_PROTECTION_MAX_SYSTEM_CPU_USAGE_PERCENTAGE_THRESHOLDS",
			Value: "0.7",
		},
		{
			Name:  "MASTER_SERVER_LOAD_PROTECTION_MAX_SYSTEM_MEMORY_USAGE_PERCENTAGE_THRESHOLDS",
			Value: "0.7",
		},
		{
			Name:  "MASTER_STATE_WHEEL_INTERVAL",
			Value: "5s",
		},
		{
			Name:  "MASTER_TASK_COMMIT_INTERVAL",
			Value: "1s",
		},
		{
			Name:  "MASTER_TASK_COMMIT_RETRYTIMES",
			Value: "5",
		},
	}

	// db env
	envs = append(envs, common.MakeDataBaseEnvs(c.dbParams)...)
	return envs
}

func (c *ContainerBuilder) VolumeMount() []corev1.VolumeMount {
	return []corev1.VolumeMount{
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
			HTTPGet: &corev1.HTTPGetAction{
				Host: "localhost",
				Path: "/actuator/health/liveness",
				Port: intstr.FromInt32(dolphinv1alpha1.MasterActualPort),
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
			HTTPGet: &corev1.HTTPGetAction{
				Host: "localhost",
				Path: "/actuator/health/readiness",
				Port: intstr.FromInt32(dolphinv1alpha1.MasterActualPort),
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
	return string(ContainerMaster)
}

const ContainerMaster resource.ContainerComponent = resource.ContainerComponent(core.Master)
