package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/apimachinery/pkg/runtime"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

type ClusterSudoRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SudoRequestSpec   `json:"spec,omitempty"`
	Status SudoRequestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type ClusterSudoRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterSudoRequest `json:"items"`
}
//
// type ClusterSudoRequestStatus struct {
//     State         string          `json:"state,omitempty"`
//     RequestID     string          `json:"requestID,omitempty"`
//     ErrorMessage  string          `json:"errorMessage,omitempty"`
//     CreatedAt     *metav1.Time    `json:"createdAt,omitempty"`
//     ExpiresAt     *metav1.Time    `json:"expiresAt,omitempty"`
//     ChildResource []ChildResource `json:"childResource,omitempty"`
// }
//
// func (in *ClusterSudoRequestStatus) DeepCopyInto(out *ClusterSudoRequestStatus) {
// 	if in == nil {
// 		return
// 	}
// 	*out = *in
// 	if in.CreatedAt != nil {
// 		in, out := &in.CreatedAt, &out.CreatedAt
// 		*out = (*in).DeepCopy()
// 	}
// 	if in.ExpiresAt != nil {
// 		in, out := &in.ExpiresAt, &out.ExpiresAt
// 		*out = (*in).DeepCopy()
// 	}
// }


// // DeepCopyObject is implemented by deepcopy-gen.
// func (in *ClusterSudoRequest) DeepCopyObject() runtime.Object {
//     if in == nil {
//         return nil
//     }
//     out := new(ClusterSudoRequest)
//     in.DeepCopyInto(out)
//     return out
// }
//

