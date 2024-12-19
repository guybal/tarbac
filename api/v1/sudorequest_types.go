package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
//     "k8s.io/apimachinery/pkg/runtime"
)

// SudoRequestSpec defines the desired state of SudoRequest
type SudoRequestSpec struct {
	Duration string `json:"duration"` // e.g., "1h" for one hour
	Policy   string `json:"policy"`   // Name of the SudoPolicy to enforce
}

// SudoRequestStatus defines the observed state of SudoRequest
type SudoRequestStatus struct {
	State     string       `json:"state,omitempty"`     // Current state: Pending, Approved, Expired
	CreatedAt *metav1.Time `json:"createdAt,omitempty"` // Timestamp when the request was created
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"` // Timestamp when the request will expire
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

type SudoRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SudoRequestSpec   `json:"spec,omitempty"`
	Status SudoRequestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type SudoRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SudoRequest `json:"items"`
}

func (in *SudoRequestStatus) DeepCopyInto(out *SudoRequestStatus) {
	if in == nil {
		return
	}
	*out = *in
	if in.CreatedAt != nil {
		in, out := &in.CreatedAt, &out.CreatedAt
		*out = (*in).DeepCopy()
	}
	if in.ExpiresAt != nil {
		in, out := &in.ExpiresAt, &out.ExpiresAt
		*out = (*in).DeepCopy()
	}
}

