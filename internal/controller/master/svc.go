package master

import (
	dolphinv1alpha1 "github.com/zncdata-labs/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/resource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const svcMasterPort = 5678
const svcActuatorPort = 5679

func NewMasterServiceHeadless(
	scheme *runtime.Scheme,
	instance *dolphinv1alpha1.DolphinschedulerCluster,
	client client.Client,
	groupName string,
	labels map[string]string,
	mergedCfg *dolphinv1alpha1.MasterRoleGroupSpec,
) *resource.GenericServiceReconciler[*dolphinv1alpha1.DolphinschedulerCluster, *dolphinv1alpha1.MasterRoleGroupSpec] {
	headlessType := resource.HeadlessService
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
			Name:       dolphinv1alpha1.MasterPortName,
			Port:       svcMasterPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString(dolphinv1alpha1.MasterPortName),
		},
		{
			Name:       dolphinv1alpha1.MasterActualPortName,
			Port:       svcActuatorPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString(dolphinv1alpha1.MasterActualPortName),
		},
	}
}
