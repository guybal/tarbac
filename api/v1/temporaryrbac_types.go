package v1

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TemporaryRBACSpec defines the desired state of TemporaryRBAC
type TemporaryRBACSpec struct {
	Subjects []rbacv1.Subject     `json:"subjects,omitempty"` // Subjects
	RoleRef  rbacv1.RoleRef       `json:"roleRef"`            // Role or ClusterRole reference
	Duration string               `json:"duration"`           // Duration for the TemporaryRBAC
    RetentionPolicy string           `json:"retentionPolicy,omitempty"` // delete or retain
}

// ChildResource represents details of the associated RoleBinding or ClusterRoleBinding
type ChildResource struct {
	APIVersion string `json:"apiVersion,omitempty"` // API version of the child resource
	Name       string `json:"name"`                // Name of the child resource
	Namespace  string `json:"namespace"`           // Namespace of the child resource
	Kind       string `json:"kind"`                // Kind of the child resource
}

// TemporaryRBACStatus defines the observed state of TemporaryRBAC
type TemporaryRBACStatus struct {
	State         string         `json:"state,omitempty"`         // State of the TemporaryRBAC
	ExpiresAt     *metav1.Time   `json:"expiresAt,omitempty"`     // Expiration time
	CreatedAt     *metav1.Time   `json:"createdAt,omitempty"`     // Creation time
	ChildResource []ChildResource `json:"childResource,omitempty"` // Details of the associated resource
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// TemporaryRBAC is the Schema for the TemporaryRBAC API
type TemporaryRBAC struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TemporaryRBACSpec   `json:"spec,omitempty"`
	Status TemporaryRBACStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TemporaryRBACList contains a list of TemporaryRBAC
type TemporaryRBACList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TemporaryRBAC `json:"items"`
}

// RoleRefWithNamespace adds an optional namespace field to the RoleRef
type RoleRefWithNamespace struct {
	rbacv1.RoleRef
	Namespace string `json:"namespace,omitempty"` // Optional namespace for namespaced bindings
}

// DeepCopyInto manually implements the deepcopy function for TemporaryRBACSpec.
func (in *TemporaryRBACSpec) DeepCopyInto(out *TemporaryRBACSpec) {
	*out = *in
	if in.Subjects != nil {
		in, out := &in.Subjects, &out.Subjects
		*out = make([]rbacv1.Subject, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy manually implements the deepcopy function for TemporaryRBACSpec.
func (in *TemporaryRBACSpec) DeepCopy() *TemporaryRBACSpec {
	if in == nil {
		return nil
	}
	out := new(TemporaryRBACSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto manually implements the deepcopy function for TemporaryRBACStatus.
// DeepCopyInto manually implements the deepcopy function for TemporaryRBACStatus.
func (in *TemporaryRBACStatus) DeepCopyInto(out *TemporaryRBACStatus) {
	*out = *in

	if in.ExpiresAt != nil {
		in, out := &in.ExpiresAt, &out.ExpiresAt
		*out = (*in).DeepCopy()
	}

	if in.CreatedAt != nil {
		in, out := &in.CreatedAt, &out.CreatedAt
		*out = (*in).DeepCopy()
	}

	if in.ChildResource != nil {
		in, out := &in.ChildResource, &out.ChildResource
		*out = make([]ChildResource, len(*in)) // Allocate new slice
		for i := range *in {
			(*out)[i] = (*in)[i] // DeepCopy each element in the slice
		}
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

// DeepCopyInto manually implements the deepcopy function for ChildResource.
func (in *ChildResource) DeepCopyInto(out *ChildResource) {
	*out = *in
}

// DeepCopy manually implements the deepcopy function for ChildResource.
func (in *ChildResource) DeepCopy() *ChildResource {
	if in == nil {
		return nil
	}
	out := new(ChildResource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto manually implements the deepcopy function for TemporaryRBAC.
func (in *TemporaryRBAC) DeepCopyInto(out *TemporaryRBAC) {
	*out = *in
	out.TypeMeta = in.TypeMeta // TypeMeta doesn't require deepcopy
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy manually implements the deepcopy function for TemporaryRBAC.
func (in *TemporaryRBAC) DeepCopy() *TemporaryRBAC {
	if in == nil {
		return nil
	}
	out := new(TemporaryRBAC)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject manually implements the deepcopy function for runtime.Object.
func (in *TemporaryRBAC) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto manually implements the deepcopy function for TemporaryRBACList.
func (in *TemporaryRBACList) DeepCopyInto(out *TemporaryRBACList) {
	*out = *in
	out.TypeMeta = in.TypeMeta // TypeMeta doesn't require deepcopy
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]TemporaryRBAC, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy manually implements the deepcopy function for TemporaryRBACList.
func (in *TemporaryRBACList) DeepCopy() *TemporaryRBACList {
	if in == nil {
		return nil
	}
	out := new(TemporaryRBACList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject manually implements the deepcopy function for runtime.Object.
func (in *TemporaryRBACList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}
