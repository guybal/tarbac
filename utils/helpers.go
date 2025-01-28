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

func trimUID(uid string, length int) string {
	trimmedUID := uid
	if len(uid) > length {
		trimmedUID = uid[len(uid)-length:]
	}
	return trimmedUID
}

// GenerateBindingName generates a unique name for the binding, limited to 63 characters.
func GenerateBindingName(subject rbacv1.Subject, roleRef rbacv1.RoleRef, uid string) string {
	// Trim components to fit within 63 characters.
	kind := truncate(strings.ToLower(subject.Kind), 10) // Max 10 characters for Kind.
	name := truncate(subject.Name, 20)                  // Max 20 characters for Subject Name.
	role := truncate(roleRef.Name, 20)                  // Max 20 characters for Role Name.
	trimmedUID := trimUID(uid, 12)                      // Always use the last 12 characters of UID.
	return fmt.Sprintf("%s-%s-%s-%s", kind, name, role, trimmedUID)
}

// GenerateTempRBACName generates a unique name for Temporary RBAC resources, limited to 63 characters.
func GenerateTempRBACName(subject rbacv1.Subject, sudoPolicy string, uid string) string {
	// Trim components to fit within 63 characters.
	kind := truncate(strings.ToLower(subject.Kind), 10) // Max 10 characters for Kind.
	name := truncate(subject.Name, 20)                  // Max 20 characters for Subject Name.
	policy := truncate(sudoPolicy, 20)                  // Max 20 characters for SudoPolicy Name.
	trimmedUID := trimUID(uid, 12)                      // Always use the last 12 characters of UID.
	return fmt.Sprintf("%s-%s-%s-%s", kind, name, policy, trimmedUID)
}

func LogInfoUID(logger logr.Logger, message string, requestID string, additionalFields ...interface{}) {
	fields := append([]interface{}{"requestID", requestID}, additionalFields...)
	logger.Info(message,
		fields...,
	)
}

func LogInfo(logger logr.Logger, message string, additionalFields ...interface{}) {
	logger.Info(message,
		additionalFields...,
	)
}

// LogError standardizes error logging across controllers.
func LogErrorUID(logger logr.Logger, err error, message string, requestID string, additionalFields ...interface{}) {
	fields := append([]interface{}{"requestID", requestID}, additionalFields...)
	logger.Error(err,
		message,
		fields...,
	)
}

func LogError(logger logr.Logger, err error, message string, additionalFields ...interface{}) {
	logger.Error(err,
		message,
		additionalFields...)
}

// FormatEventMessage formats an event message with the requestId if applicable.
func FormatEventMessage(message, requestId string) string {
	if requestId != "" {
		return fmt.Sprintf("%s [RequestID: %s]", message, requestId)
	}
	return message
}
