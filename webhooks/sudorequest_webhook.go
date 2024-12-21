package webhooks

import (
	"context"
	"net/http"
	"encoding/json"

	v1 "github.com/guybal/tarbac/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type SudoRequestAnnotator struct {
	decoder *admission.Decoder
}

// Handle implements the admission.Handler interface for mutating webhooks
func (a *SudoRequestAnnotator) Handle(ctx context.Context, req admission.Request) admission.Response {
	if req.Kind.Kind != "SudoRequest" && req.Kind.Kind != "ClusterSudoRequest" {
		return admission.Allowed("Ignoring non-SudoRequest or non-ClusterSudoRequest resource")
	}

	// Decode the incoming object
    var sudoRequest v1.SudoRequest
    if err := json.Unmarshal(req.Object.Raw, &sudoRequest); err != nil {
        return admission.Errored(http.StatusBadRequest, err)
    }

	// Annotate with the requester information
	if sudoRequest.Annotations == nil {
		sudoRequest.Annotations = map[string]string{}
	}
	sudoRequest.Annotations["tarbac.io/requester"] = req.UserInfo.Username

	// Marshal the updated object
	mutatedObject, err := runtime.Encode(serializer.NewCodecFactory(runtime.NewScheme()).LegacyCodec(v1.GroupVersion), &sudoRequest)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, mutatedObject)
}

// InjectDecoder injects the decoder into the webhook
func (a *SudoRequestAnnotator) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d
	return nil
}
