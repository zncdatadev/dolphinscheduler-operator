package api

import (
	dolphinv1alpha1 "github.com/zncdata-labs/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/common"
	corev1 "k8s.io/api/core/v1"
	"strconv"
)

func NewApiContainerBuilder(
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
		ContainerBuilder:        *common.NewContainerBuilder(image, imagePullPolicy),
		zookeeperDiscoveryZNode: zookeeperDiscoveryZNode,
		resourceSpec:            resourceSpec,
		envConfigName:           envConfigName,
		configConfigMapName:     configConfigMapName,
		dbSpec:                  dbSpec,
	}
}

type ContainerApiBuilderType interface {
	common.ContainerName
	common.ContainerEnv
	common.ContainerEnvFrom
	common.VolumeMount
	common.LivenessProbe
	common.ReadinessProbe
	common.ContainerPorts
}

var _ ContainerApiBuilderType = &ContainerBuilder{}

type ContainerBuilder struct {
	common.ContainerBuilder
	zookeeperDiscoveryZNode string
	resourceSpec            *dolphinv1alpha1.ResourcesSpec
	envConfigName           string
	configConfigMapName     string
	dbSpec                  *dolphinv1alpha1.DatabaseSpec
}

func (c *ContainerBuilder) ContainerPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			ContainerPort: dolphinv1alpha1.ApiPort,
			Name:          dolphinv1alpha1.ApiPortName,
			Protocol:      corev1.ProtocolTCP,
		},
		{
			ContainerPort: dolphinv1alpha1.ApiPythonPort,
			Name:          dolphinv1alpha1.ApiPythonPortName,
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
					Key: common.ZookeeperDiscoveryKey,
				},
			},
		},
		{
			Name:  "JAVA_OPTS",
			Value: "-Xms512m -Xmx512m -Xmn256m",
		},
	}
	envs = append(envs, common.MakeDataBaseEnvs(c.dbSpec)...)
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
			Exec: &corev1.ExecAction{
				Command: []string{
					"curl",
					"-s",
					"http://localhost:" + strconv.Itoa(dolphinv1alpha1.ApiPort) + "/dolphinscheduler/actuator/health/liveness",
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
					"http://localhost:" + strconv.Itoa(dolphinv1alpha1.ApiPort) + "/dolphinscheduler/actuator/health/readiness",
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
	return string(common.Api)
}
