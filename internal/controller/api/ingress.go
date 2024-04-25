package api

import (
	"context"
	dolphinv1alpha1 "github.com/zncdata-labs/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/common"
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
	mergedCfg *dolphinv1alpha1.RoleGroupSpec,
) *IngressReconciler {
	return &IngressReconciler{
		GeneralResourceStyleReconciler: *common.NewGeneraResourceStyleReconciler(
			scheme,
			instance,
			client,
			groupName,
			labels,
			mergedCfg,
		),
	}
}

var _ common.ResourceBuilder = &IngressReconciler{}

type IngressReconciler struct {
	common.GeneralResourceStyleReconciler[*dolphinv1alpha1.DolphinschedulerCluster, *dolphinv1alpha1.RoleGroupSpec]
}

func (i *IngressReconciler) Build(ctx context.Context) (client.Object, error) {
	return &v1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dolphinscheduler",
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
												Name: "api-port",
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
