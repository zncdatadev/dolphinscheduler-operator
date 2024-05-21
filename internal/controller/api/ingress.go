package api

import (
	"context"

	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/core"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewIngress(
	scheme *runtime.Scheme,
	instance *dolphinv1alpha1.DolphinschedulerCluster,
	client client.Client,
	groupName string,
	labels map[string]string,
	mergedCfg *dolphinv1alpha1.ApiRoleGroupSpec,
) *IngressReconciler {
	return &IngressReconciler{
		GeneralResourceStyleReconciler: *core.NewGeneraResourceStyleReconciler(
			scheme,
			instance,
			client,
			groupName,
			labels,
			mergedCfg,
		),
	}
}

var _ core.ResourceBuilder = &IngressReconciler{}

type IngressReconciler struct {
	core.GeneralResourceStyleReconciler[*dolphinv1alpha1.DolphinschedulerCluster, *dolphinv1alpha1.ApiRoleGroupSpec]
}

func (i *IngressReconciler) Build(ctx context.Context) (client.Object, error) {
	return &v1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingressName(i.Instance.GetName(), i.GroupName),
			Namespace: i.Instance.Namespace,
			Labels:    i.Labels,
		},
		Spec: v1.IngressSpec{
			Rules: []v1.IngressRule{
				{
					Host: i.Instance.Spec.ClusterConfigSpec.IngressHost,
					IngressRuleValue: v1.IngressRuleValue{
						HTTP: &v1.HTTPIngressRuleValue{
							Paths: []v1.HTTPIngressPath{
								{
									Backend: v1.IngressBackend{
										Service: &v1.IngressServiceBackend{
											Name: createSvcName(i.Instance.GetName(), i.GroupName),
											Port: v1.ServiceBackendPort{
												Name: dolphinv1alpha1.ApiPortName,
											},
										},
									},
									Path:     "/dolphinscheduler",
									PathType: func() *v1.PathType { p := v1.PathTypePrefix; return &p }(),
								},
							},
						},
					},
				},
			},
		},
	}, nil
}
