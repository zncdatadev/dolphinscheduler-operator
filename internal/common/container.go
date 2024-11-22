package common

import (
	"strconv"
	"strings"

	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/constant"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/util"
	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/builder"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	opgoutil "github.com/zncdatadev/operator-go/pkg/util"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

var contaienrBuilderLogger = ctrl.Log.WithName("container-builder")

func NewContainerBuilder(
	contaienr util.ContainerComponent,
	image *opgoutil.Image,
	zookeeperConfigMapName string,
	roleGroupInfo *reconciler.RoleGroupInfo,
	roleGroupConfig *commonsv1alpha1.RoleGroupConfigSpec,
) *ContainerBuilder {
	b := &ContainerBuilder{
		Container:              builder.NewContainer(string(contaienr), image),
		ZookeeperConfigMapName: zookeeperConfigMapName,
		RoleGroupInfo:          roleGroupInfo,
		RoleGroupConfig:        roleGroupConfig,
	}
	b.volumeMounts = b.commonVolumeMounts()

	return b
}

type ContainerBuilder struct {
	*builder.Container
	ZookeeperConfigMapName string
	RoleGroupInfo          *reconciler.RoleGroupInfo
	RoleGroupConfig        *commonsv1alpha1.RoleGroupConfigSpec

	secretEnvfrom string
	envs          []corev1.EnvVar
	ports         []corev1.ContainerPort
	readiness     *corev1.Probe
	liveness      *corev1.Probe
	volumeMounts  []corev1.VolumeMount
	args          []string
	argsScript    string
}

// with envs
// key is env name, value is env value
func (c *ContainerBuilder) WithEnvs(envMap util.SortedMap) *ContainerBuilder {
	// convert map to envs
	var envs []corev1.EnvVar
	envMap.Range(func(k string, v interface{}) bool {
		switch envVal := v.(type) {
		case string:
			envs = append(envs, corev1.EnvVar{
				Name:  k,
				Value: envVal,
			})
		case corev1.EnvVarSource:
			envs = append(envs, corev1.EnvVar{
				Name:      k,
				ValueFrom: &envVal,
			})
		}
		return true
	})
	envs = append(envs, corev1.EnvVar{
		Name: "REGISTRY_ZOOKEEPER_CONNECT_STRING",
		ValueFrom: &corev1.EnvVarSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: c.ZookeeperConfigMapName,
				},
				Key: constant.ZookeeperDiscoveryKey,
			},
		},
	})
	c.envs = append(c.envs, envs...)
	return c
}

// with secret env from
// key is envfrom name, value is envfrom value
func (c *ContainerBuilder) WithSecretEnvFrom(secretName string) *ContainerBuilder {
	c.secretEnvfrom = secretName
	return c
}

// env map from secret
func (c *ContainerBuilder) AddEnvFromSecret(secretName string, envToSecretKeymap map[string]string) *ContainerBuilder {

	for envKey, secretKey := range envToSecretKeymap {
		c.envs = append(c.envs, corev1.EnvVar{
			Name: envKey,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: secretKey,
				},
			},
		})
	}
	return c
}

// get command args
func (c *ContainerBuilder) GetCommandArgs() []string {
	return c.args
}

// with command args
func (c *ContainerBuilder) CommonCommandArgs() *ContainerBuilder {
	var args []string
	args = append(args, opgoutil.CommonBashTrapFunctions)
	args = append(args, opgoutil.RemoveVectorShutdownFileCommand())
	args = append(args, opgoutil.InvokePrepareSignalHandlers)
	args = append(args, RoleExecArgs(c.RoleGroupInfo.RoleName))
	args = append(args, opgoutil.InvokeWaitForTermination)
	args = append(args, opgoutil.CreateVectorShutdownFileCommand())
	c.argsScript = strings.Join(args, "\n")
	c.args = append(c.args, args...)
	return c
}

// reset command args
func (c *ContainerBuilder) ResetCommandArgs(script string) *ContainerBuilder {
	c.argsScript = script
	return c
}

// with ports
// key is port name, value is port
func (c *ContainerBuilder) WithPorts(portMap util.SortedMap) *ContainerBuilder {
	// convert map to ports
	var ports []corev1.ContainerPort
	portMap.Range(func(k string, v interface{}) bool {
		port, err := ToContainerPortInt32(v)
		if err != nil {
			contaienrBuilderLogger.Error(err, "convert port to int32 failed")
			return false
		}
		ports = append(ports, corev1.ContainerPort{
			ContainerPort: port,
			Name:          k,
			Protocol:      corev1.ProtocolTCP,
		})
		return true
	})
	c.ports = append(c.ports, ports...)
	return c
}

// with readiness probe and liveness probe
// port is service port
func (c *ContainerBuilder) WithReadinessAndLivenessProbe(port int) *ContainerBuilder {
	readinessProbe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{
					"curl",
					// "-s",
					"http://localhost:" + strconv.Itoa(port) + "/actuator/health/readiness",
				},
			},
		},
		InitialDelaySeconds: 30,
		SuccessThreshold:    1,
		FailureThreshold:    3,
		PeriodSeconds:       30,
		TimeoutSeconds:      5,
	}
	c.readiness = readinessProbe

	livenessProbe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{
					"curl",
					// "-s",
					"http://localhost:" + strconv.Itoa(port) + "/actuator/health/liveness",
				},
			},
		},
		InitialDelaySeconds: 30,
		SuccessThreshold:    1,
		FailureThreshold:    3,
		PeriodSeconds:       30,
		TimeoutSeconds:      5,
	}
	c.liveness = livenessProbe
	return c
}

// with volume mounts
// key is volume name, value is mount path
func (c *ContainerBuilder) WithVolumeMounts(vmMap util.SortedMap) *ContainerBuilder {
	if len(vmMap) != 0 {
		vmMap.Range(func(k string, v interface{}) bool {
			c.volumeMounts = append(c.volumeMounts, corev1.VolumeMount{
				Name:      k,
				MountPath: v.(string),
			})
			return true
		})
	}
	return c
}

// reset volume mounts
func (c *ContainerBuilder) ResetVolumeMounts() *ContainerBuilder {
	c.volumeMounts = []corev1.VolumeMount{}
	return c
}

// common volume mounts
func (c *ContainerBuilder) commonVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      dolphinv1alpha1.ConfigVolumeName,
			MountPath: RoleConfigPath(util.Role(c.RoleGroupInfo.RoleName), dolphinv1alpha1.DolphinCommonPropertiesName),
			SubPath:   dolphinv1alpha1.DolphinCommonPropertiesName,
		},
		{
			Name:      dolphinv1alpha1.LogbackVolumeName,
			MountPath: RoleConfigPath(util.Role(c.RoleGroupInfo.RoleName), dolphinv1alpha1.LogbackPropertiesFileName),
			SubPath:   dolphinv1alpha1.LogbackPropertiesFileName,
		},
	}
}

// build container
func (c *ContainerBuilder) Build() *corev1.Container {
	c.SetLivenessProbe(c.liveness).
		SetReadinessProbe(c.readiness).
		AddEnvVars(c.envs).
		AddEnvFromConfigMap(RoleGroupEnvsConfigMapName(c.RoleGroupInfo.GetClusterName())).
		AddPorts(c.ports).
		SetCommand([]string{"/bin/bash", "-x", "-euo", "pipefail", "-c"}).
		AddVolumeMounts(c.volumeMounts).
		SetArgs([]string{c.argsScript})
	if c.secretEnvfrom != "" {
		c.WithSecretEnvFrom(c.secretEnvfrom)
	}
	if c.RoleGroupConfig != nil {
		c.SetResources(c.RoleGroupConfig.Resources)
	}
	return c.Container.Build()
}
