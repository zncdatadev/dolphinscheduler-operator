package alerter

import (
	dolphinv1alpha1 "github.com/zncdata-labs/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/common"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/core"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/resource"
	corev1 "k8s.io/api/core/v1"
	"strconv"
)

func NewAlerterContainerBuilder(
	image string,
	imagePullPolicy corev1.PullPolicy,
	zookeeperDiscoveryZNode string,
	resourceSpec *dolphinv1alpha1.ResourcesSpec,
	envConfigName string,
	configConfigMapName string,
	dbParams *resource.DatabaseParams,
) *ContainerBuilder {
	return &ContainerBuilder{
		ContainerBuilder:        *resource.NewContainerBuilder(image, imagePullPolicy),
		zookeeperDiscoveryZNode: zookeeperDiscoveryZNode,
		resourceSpec:            resourceSpec,
		envConfigName:           envConfigName,
		configConfigMapName:     configConfigMapName,
		dbParams:                dbParams,
	}
}

type ContainerAlerterBuilderType interface {
	resource.ContainerName
	resource.ContainerEnv
	resource.ContainerEnvFrom
	resource.VolumeMount
	resource.LivenessProbe
	resource.ReadinessProbe
	resource.ContainerPorts
}

var _ ContainerAlerterBuilderType = &ContainerBuilder{}

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
			ContainerPort: dolphinv1alpha1.AlerterPort,
			Name:          dolphinv1alpha1.AlerterPortName,
			Protocol:      corev1.ProtocolTCP,
		},
		{
			ContainerPort: dolphinv1alpha1.AlerterActualPort,
			Name:          dolphinv1alpha1.AlerterActualPortName,
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
			Value: "-Xms512m -Xmx512m -Xmn256m",
		},
	}
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
			Exec: &corev1.ExecAction{
				Command: []string{
					"curl",
					"-s",
					"http://localhost:" + strconv.Itoa(dolphinv1alpha1.AlerterActualPort) + "/actuator/health/liveness",
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
					"http://localhost:" + strconv.Itoa(dolphinv1alpha1.AlerterActualPort) + "/actuator/health/readiness",
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
	return string(ContainerAlerter)
}

const ContainerAlerter resource.ContainerComponent = resource.ContainerComponent(core.Alerter)
