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
	saName := builder.ServiceAccountName(dolphinv1alpha1.DefaultProductName)
	saBuilder := builder.NewGenericServiceAccountBuilder(&client, saName, lables, nil)
	return reconciler.NewGenericResourceReconciler(&client, saName, saBuilder)
}

func NewRoleReconciler(
	client client.Client,
	lables map[string]string,
	annotations map[string]string,
) reconciler.ResourceReconciler[*builder.GenericRoleBuilder] {
	roleName := builder.RoleName(dolphinv1alpha1.DefaultProductName)
	roleBuilder := builder.NewGenericRoleBuilder(&client, roleName, lables, nil)
	roleBuilder.AddPolicyRules([]rbacv1.PolicyRule{
		{
			Verbs:     []string{"get", "list", "watch"},
			APIGroups: []string{""},
			Resources: []string{"configmaps"},
		},
	})
	return reconciler.NewGenericResourceReconciler(&client, roleName, roleBuilder)
}

func NewRoleBindingReconciler(
	client client.Client,
	lables map[string]string,
	annotations map[string]string,
) reconciler.ResourceReconciler[*builder.GenericRoleBindingBuilder] {
	roleBindingName := builder.RoleBindingName(dolphinv1alpha1.DefaultProductName)
	roleBindingBuilder := builder.NewGenericRoleBindingBuilder(&client, roleBindingName, lables, nil)
	roleBindingBuilder.AddSubject(builder.ServiceAccountName(dolphinv1alpha1.DefaultProductName))
	roleBindingBuilder.SetRoleRef(builder.RoleName(dolphinv1alpha1.DefaultProductName), false)
	return reconciler.NewGenericResourceReconciler(&client, roleBindingName, roleBindingBuilder)
}
