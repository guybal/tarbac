package v1

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// 	"k8s.io/apimachinery/pkg/runtime"
)

// RoleRef defines a reference to a role
// (either Role or ClusterRole)
// type RoleRef struct {
// 	APIGroup string `json:"apiGroup"` // The API Version of the referenced role
// 	Kind     string `json:"kind"`     // The kind of role (Role or ClusterRole)
// 	Name     string `json:"name"`     // The name of the referenced role
// }

// SudoPolicySpec defines the desired state of SudoPolicy
type SudoPolicySpec struct {
	MaxDuration               string                `json:"maxDuration"`                         // Maximum allowed duration
	RoleRef                   rbacv1.RoleRef        `json:"roleRef"`                             // Role or ClusterRole reference
	AllowedUsers              []UserRef             `json:"allowedUsers"`                        // List of allowed users
	AllowedNamespaces         []string              `json:"allowedNamespaces,omitempty"`         // Specific namespaces
	AllowedNamespacesSelector *metav1.LabelSelector `json:"allowedNamespacesSelector,omitempty"` // Namespace selector
}

// UserRef defines a reference to a user
// allowed to request sudo access
type UserRef struct {
	Name string `json:"name"` // Name of the allowed user
}

// SudoPolicyStatus defines the observed state of SudoPolicy
type SudoPolicyStatus struct {
	State        string `json:"state,omitempty"` // Current state of the policy
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

type SudoPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SudoPolicySpec   `json:"spec,omitempty"`
	Status SudoPolicyStatus `json:"status,omitempty"`
}

//
// func (in *SudoPolicy) DeepCopyObject() runtime.Object {
// 	if c := in.DeepCopy(); c != nil {
// 		return c
// 	}
// 	return nil
// }

// +kubebuilder:object:root=true

type SudoPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SudoPolicy `json:"items"`
}

//
// func (in *SudoPolicyList) DeepCopyObject() runtime.Object {
// 	if c := in.DeepCopy(); c != nil {
// 		return c
// 	}
// 	return nil
// }
//
// // RoleRef DeepCopyInto implementation
// func (in *RoleRef) DeepCopyInto(out *RoleRef) {
// 	*out = *in
// }

// func (in *SudoPolicySpec) DeepCopyInto(out *SudoPolicySpec) {
// 	*out = *in
// 	in.RoleRef.DeepCopyInto(&out.RoleRef)
// 	if in.AllowedUsers != nil {
// 		in, out := &in.AllowedUsers, &out.AllowedUsers
// 		*out = make([]UserRef, len(*in))
// 		copy(*out, *in)
// 	}
// 	if in.AllowedNamespaces != nil {
// 		in, out := &in.AllowedNamespaces, &out.AllowedNamespaces
// 		*out = make([]string, len(*in))
// 		copy(*out, *in)
// 	}
// 	if in.AllowedNamespacesSelector != nil {
// 		in, out := &in.AllowedNamespacesSelector, &out.AllowedNamespacesSelector
// 		*out = (*in).DeepCopy()
// 	}
// }

func (in *SudoPolicySpec) DeepCopyInto(out *SudoPolicySpec) {
	*out = *in
	out.RoleRef = in.RoleRef // rbacv1.RoleRef is already a simple struct; no deep copy needed.
	if in.AllowedUsers != nil {
		in, out := &in.AllowedUsers, &out.AllowedUsers
		*out = make([]UserRef, len(*in))
		copy(*out, *in)
	}
	if in.AllowedNamespaces != nil {
		in, out := &in.AllowedNamespaces, &out.AllowedNamespaces
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.AllowedNamespacesSelector != nil {
		in, out := &in.AllowedNamespacesSelector, &out.AllowedNamespacesSelector
		*out = (*in).DeepCopy()
	}
}
