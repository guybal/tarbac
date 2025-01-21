package utils

import (
	"fmt"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
)

// GenerateBindingName generates a unique name for the binding
func GenerateBindingName(subject rbacv1.Subject, roleRef rbacv1.RoleRef, uid string) string {
    trimmedUID := uid
    if len(uid) > 10 {
        trimmedUID = uid[len(uid)-10:]
    }
	return fmt.Sprintf("%s-%s-%s-%s", strings.ToLower(subject.Kind), subject.Name, roleRef.Name, trimmedUID)
}

func GenerateTempRBACName(subject rbacv1.Subject, sudoPolicy string, uid string) string {
    trimmedUID := uid
    if len(uid) > 10 {
        trimmedUID = uid[len(uid)-10:]
    }
	return fmt.Sprintf("%s-%s-%s-%s", strings.ToLower(subject.Kind), subject.Name, sudoPolicy, trimmedUID)
}
