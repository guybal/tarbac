package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

// TemporaryRBACSpec defines the desired state of TemporaryRBAC
type TemporaryRBACSpec struct {
	Subject   rbacv1.Subject `json:"subject"`
	RoleRef   rbacv1.RoleRef `json:"roleRef"`
	Duration  string         `json:"duration"`
}

// TemporaryRBACStatus defines the observed state of TemporaryRBAC
type TemporaryRBACStatus struct {
	ExpiresAt metav1.Time `json:"expiresAt,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type TemporaryRBAC struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TemporaryRBACSpec   `json:"spec,omitempty"`
	Status TemporaryRBACStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type TemporaryRBACList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TemporaryRBAC `json:"items"`
}
