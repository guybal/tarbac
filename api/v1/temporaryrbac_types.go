package v1

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TemporaryRBACSpec defines the desired state of TemporaryRBAC
type TemporaryRBACSpec struct {
	Subject  rbacv1.Subject `json:"subject"`
	RoleRef  rbacv1.RoleRef `json:"roleRef"`
	Duration string         `json:"duration"`
}

type ChildResource struct {
    APIVersion string `json:"apiVersion,omitempty"` // Add this field
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Kind      string `json:"kind"`
}

// TemporaryRBACStatus defines the observed state of TemporaryRBAC
type TemporaryRBACStatus struct {
	State     string      `json:"state,omitempty"`
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`
    CreatedAt *metav1.Time `json:"createdAt,omitempty"`
//     TimeToLive       string      `json:"timeToLive,omitempty"`
	ChildResource *ChildResource      `json:"childResource,omitempty"`
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

// DeepCopyInto manually implements the deepcopy function for TemporaryRBACStatus.
func (in *TemporaryRBACStatus) DeepCopyInto(out *TemporaryRBACStatus) {
	*out = *in
	if in.ExpiresAt != nil {
		in, out := &in.ExpiresAt, &out.ExpiresAt
		*out = (*in).DeepCopy()
	}
}

// DeepCopy manually implements the deepcopy function for TemporaryRBACStatus.
func (in *TemporaryRBACStatus) DeepCopy() *TemporaryRBACStatus {
	if in == nil {
		return nil
	}
	out := new(TemporaryRBACStatus)
	in.DeepCopyInto(out)
	return out
}

