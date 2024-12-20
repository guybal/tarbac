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


