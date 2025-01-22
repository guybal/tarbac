package utils

import (
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	// ctrl "sigs.k8s.io/controller-runtime"
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
	kind := truncate(strings.ToLower(subject.Kind), 10) // Max 10 characters for Kind.
	name := truncate(subject.Name, 20)                  // Max 20 characters for Subject Name.
	role := truncate(roleRef.Name, 20)                  // Max 20 characters for Role Name.
	trimmedUID := truncate(uid, 12)                     // Always use the last 12 characters of UID.
	return fmt.Sprintf("%s-%s-%s-%s", kind, name, role, trimmedUID)
}

// GenerateTempRBACName generates a unique name for Temporary RBAC resources, limited to 63 characters.
func GenerateTempRBACName(subject rbacv1.Subject, sudoPolicy string, uid string) string {
	// Trim components to fit within 63 characters.
	kind := truncate(strings.ToLower(subject.Kind), 10) // Max 10 characters for Kind.
	name := truncate(subject.Name, 20)                  // Max 20 characters for Subject Name.
	policy := truncate(sudoPolicy, 20)                  // Max 20 characters for SudoPolicy Name.
	trimmedUID := truncate(uid, 12)                     // Always use the last 12 characters of UID.
	return fmt.Sprintf("%s-%s-%s-%s", kind, name, policy, trimmedUID)
}

// LogInfo standardizes info logging across controllers.
// func LogInfoUID(logger logr.Logger, message string, controller string, kind string, resourceName string, namespace string, requestID string) {
// 	logger.Info(message,
// 		"controller", controller,
// 		"controllerKind", kind,
// 		kind, map[string]string{"name": resourceName},
// 		"namespace", namespace,
// 		"name", resourceName,
// 		"requestID", requestID,
// 	)
// }

func LogInfoUID(logger logr.Logger, message string, requestID string, additionalFields ...interface{}) {
	fields := append([]interface{}{"requestID", requestID}, additionalFields...)
	logger.Info(message,
		fields...,
	// "controllerKind", kind,
	// kind, map[string]string{"name": resourceName},
	// "namespace", namespace,
	// "name", resourceName,
	)
}

func LogInfo(logger logr.Logger, message string) {
	logger.Info(message) // "controller", controller,
	// "controllerKind", kind,
	// kind, map[string]string{"name": resourceName},
	// "namespace", namespace,
	// "name", resourceName,
}

// LogError standardizes error logging across controllers.
func LogErrorUID(logger logr.Logger, err error, message string, requestID string, additionalFields ...interface{}) {
	fields := append([]interface{}{"requestID", requestID}, additionalFields...)
	logger.Error(err, message, fields...,
	)
}

func LogError(logger logr.Logger, err error, message string) {
	logger.Error(err, message)
}
