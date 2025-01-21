package utils

import (
	"fmt"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
)

// truncate ensures a string doesn't exceed the given length.
func truncate(input string, maxLength int) string {
	if len(input) > maxLength {
		return input[:maxLength]
	}
	return input
}

// GenerateBindingName generates a unique name for the binding, limited to 63 characters.
func GenerateBindingName(subject rbacv1.Subject, roleRef rbacv1.RoleRef, uid string) string {
	// Trim components to fit within 63 characters.
	kind := truncate(strings.ToLower(subject.Kind), 10)          // Max 10 characters for Kind.
	name := truncate(subject.Name, 20)                          // Max 20 characters for Subject Name.
	role := truncate(roleRef.Name, 20)                          // Max 20 characters for Role Name.
	trimmedUID := truncate(uid, 12)                             // Always use the last 12 characters of UID.
	return fmt.Sprintf("%s-%s-%s-%s", kind, name, role, trimmedUID)
}

// GenerateTempRBACName generates a unique name for Temporary RBAC resources, limited to 63 characters.
func GenerateTempRBACName(subject rbacv1.Subject, sudoPolicy string, uid string) string {
	// Trim components to fit within 63 characters.
	kind := truncate(strings.ToLower(subject.Kind), 10)         // Max 10 characters for Kind.
	name := truncate(subject.Name, 20)                         // Max 20 characters for Subject Name.
	policy := truncate(sudoPolicy, 20)                         // Max 20 characters for SudoPolicy Name.
	trimmedUID := truncate(uid, 12)                            // Always use the last 12 characters of UID.
	return fmt.Sprintf("%s-%s-%s-%s", kind, name, policy, trimmedUID)
}


// package utils
//
// import (
// 	"fmt"
// 	"strings"
//
// 	rbacv1 "k8s.io/api/rbac/v1"
// )
//
// // GenerateBindingName generates a unique name for the binding
// func GenerateBindingName(subject rbacv1.Subject, roleRef rbacv1.RoleRef, uid string) string {
//     trimmedUID := uid
//     if len(uid) > 12 {
//         trimmedUID = uid[len(uid)-12:]
//     }
// 	return fmt.Sprintf("%s-%s-%s-%s", strings.ToLower(subject.Kind), subject.Name, roleRef.Name, trimmedUID)
// }
//
// func GenerateTempRBACName(subject rbacv1.Subject, sudoPolicy string, uid string) string {
//     trimmedUID := uid
//     if len(uid) > 12 {
//         trimmedUID = uid[len(uid)-12:]
//     }
// 	return fmt.Sprintf("%s-%s-%s-%s", strings.ToLower(subject.Kind), subject.Name, sudoPolicy, trimmedUID)
// }
