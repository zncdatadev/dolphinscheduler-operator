package worker

import (
	dolphinv1alpha1 "github.com/zncdata-labs/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const svcWorkerPort = 1234
const svcActuatorPort = 1335

func NewWorkerServiceHeadless(
	scheme *runtime.Scheme,
	instance *dolphinv1alpha1.DolphinschedulerCluster,
	client client.Client,
	groupName string,
	labels map[string]string,
	mergedCfg *dolphinv1alpha1.RoleGroupSpec,
) *common.GenericServiceReconciler[*dolphinv1alpha1.DolphinschedulerCluster, *dolphinv1alpha1.RoleGroupSpec] {
	headlessType := common.HeadlessService
	buidler := common.NewServiceBuilder(
		createSvcName(instance.GetName(), groupName),
		instance.GetNamespace(),
		labels,
		makeGroupSvcPorts(),
	).SetClusterIP(&headlessType)
	return common.NewGenericServiceReconciler(
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
			Name:       dolphinv1alpha1.WorkerPortName,
			Port:       svcWorkerPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString(dolphinv1alpha1.WorkerPortName),
		},
		{
			Name:       dolphinv1alpha1.WorkerActualPortName,
			Port:       svcActuatorPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString(dolphinv1alpha1.WorkerActualPortName),
		},
	}
}
