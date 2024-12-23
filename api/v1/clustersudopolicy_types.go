package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/apimachinery/pkg/runtime"
)


// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

type ClusterSudoPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SudoPolicySpec   `json:"spec,omitempty"`
	Status ClusterSudoPolicyStatus `json:"status,omitempty"`
}

// ClusterSudoPolicyStatus defines the observed state of SudoPolicy
type ClusterSudoPolicyStatus struct {
	State string `json:"state,omitempty"` // Current state of the policy
	Namespaces []string `json:"namespaces,omitempty"`
}

func (in *ClusterSudoPolicyStatus) DeepCopyInto(out *ClusterSudoPolicyStatus) {
	*out = *in
	if in.Namespaces != nil {
		in, out := &in.Namespaces, &out.Namespaces
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// func (in *ClusterSudoPolicy) DeepCopyObject() runtime.Object {
// 	if c := in.DeepCopy(); c != nil {
// 		return c
// 	}
// 	return nil
// }

// +kubebuilder:object:root=true

type ClusterSudoPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterSudoPolicy `json:"items"`
}
//
// func (in *ClusterSudoPolicyList) DeepCopyObject() runtime.Object {
// 	if c := in.DeepCopy(); c != nil {
// 		return c
// 	}
// 	return nil
// }
