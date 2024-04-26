package alerter

import (
	dolphinv1alpha1 "github.com/zncdata-labs/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/resource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const svcAlerterPort = 12345
const svcAlerterPythonPort = 25333

func NewAlerterService(
	scheme *runtime.Scheme,
	instance *dolphinv1alpha1.DolphinschedulerCluster,
	client client.Client,
	groupName string,
	labels map[string]string,
	mergedCfg *dolphinv1alpha1.RoleGroupSpec,
) *resource.GenericServiceReconciler[*dolphinv1alpha1.DolphinschedulerCluster, *dolphinv1alpha1.RoleGroupSpec] {
	headlessType := resource.Service
	buidler := resource.NewServiceBuilder(
		createSvcName(instance.GetName(), groupName),
		instance.GetNamespace(),
		labels,
		makeGroupSvcPorts(),
	).SetClusterIP(&headlessType)
	return resource.NewGenericServiceReconciler(
		scheme,
		instance,
		client,
		groupName,
		labels,
		mergedCfg,
		buidler,
	)
}

func makeGroupSvcPorts() []corev1.ServicePort {
	return []corev1.ServicePort{
		{
			Name:       dolphinv1alpha1.AlerterPortName,
			Port:       svcAlerterPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString(dolphinv1alpha1.AlerterPortName),
		},
		{
			Name:       dolphinv1alpha1.AlerterActualPortName,
			Port:       svcAlerterPythonPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString(dolphinv1alpha1.AlerterActualPortName),
		},
	}
}
