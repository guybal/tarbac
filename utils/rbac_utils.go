package utils

import (
	"fmt"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
)

// GenerateBindingName generates a unique name for the binding
func GenerateBindingName(subject rbacv1.Subject, roleRef rbacv1.RoleRef) string {
	return fmt.Sprintf("%s-%s-%s", strings.ToLower(subject.Kind), subject.Name, roleRef.Name)
}
