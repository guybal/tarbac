package v1

import (
// 	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	runtime "k8s.io/apimachinery/pkg/runtime"
)

// +kubebuilder:object:root=true
type ClusterTemporaryRBAC struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TemporaryRBACSpec   `json:"spec,omitempty"`
	Status TemporaryRBACStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type ClusterTemporaryRBACList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterTemporaryRBAC `json:"items"`
}

func (in *ClusterTemporaryRBACList) DeepCopyInto(out *ClusterTemporaryRBACList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterTemporaryRBAC, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i]) // ObjectMeta is handled as a struct here
		}
	}
}


// type ClusterTemporaryRBACSpec struct {
//     Subjects       []rbacv1.Subject `json:"subjects,omitempty"`
//     RoleRef        rbacv1.RoleRef   `json:"roleRef"`
//     Duration       string           `json:"duration"`
//     DeletionPolicy string           `json:"deletionPolicy,omitempty"`
// }

// type ClusterTemporaryRBACStatus struct {
//     State         string         `json:"state,omitempty"`
//     ExpiresAt     *metav1.Time   `json:"expiresAt,omitempty"`
//     CreatedAt     *metav1.Time   `json:"createdAt,omitempty"`
//     ChildResource []ChildResource `json:"childResources,omitempty"`
// }
//
// func (in *ClusterTemporaryRBAC) DeepCopyInto(out *ClusterTemporaryRBAC) {
//     *out = *in
//     in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
//     in.Spec.DeepCopyInto(&out.Spec)
//     in.Status.DeepCopyInto(&out.Status)
// }


