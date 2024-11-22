package cluster

import (
	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/builder"
	"github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	rbacv1 "k8s.io/api/rbac/v1"
)

func NewServiceAccountReconciler(
	client client.Client,
	lables map[string]string,
	annotations map[string]string,
) reconciler.ResourceReconciler[*builder.GenericServiceAccountBuilder] {
	saName := dolphinv1alpha1.DefaultProductName
	saBuilder := builder.NewGenericServiceAccountBuilder(&client, saName, func(o *builder.Options) {
		o.Labels = lables
	})
	return reconciler.NewGenericResourceReconciler(&client, saBuilder)
}

func NewRoleReconciler(
	client client.Client,
	lables map[string]string,
	annotations map[string]string,
) reconciler.ResourceReconciler[*builder.GenericRoleBuilder] {
	roleName := dolphinv1alpha1.DefaultProductName
	roleBuilder := builder.NewGenericRoleBuilder(&client, roleName, func(o *builder.Options) {
		o.Labels = lables
	})
	roleBuilder.AddPolicyRules([]rbacv1.PolicyRule{
		{
			Verbs:     []string{"get", "list", "watch"},
			APIGroups: []string{""},
			Resources: []string{"configmaps"},
		},
	})
	return reconciler.NewGenericResourceReconciler(&client, roleBuilder)
}

func NewRoleBindingReconciler(
	client client.Client,
	lables map[string]string,
	annotations map[string]string,
) reconciler.ResourceReconciler[*builder.GenericRoleBindingBuilder] {
	roleBindingName := dolphinv1alpha1.DefaultProductName
	roleBindingBuilder := builder.NewGenericRoleBindingBuilder(&client, roleBindingName, func(o *builder.Options) {
		o.Labels = lables
	})
	roleBindingBuilder.AddSubject(dolphinv1alpha1.DefaultProductName)
	roleBindingBuilder.SetRoleRef(dolphinv1alpha1.DefaultProductName, false)
	return reconciler.NewGenericResourceReconciler(&client, roleBindingBuilder)
}
