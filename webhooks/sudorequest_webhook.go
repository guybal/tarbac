package webhooks

import (
	"context"
	"fmt"
	"net/http"

	v1 "github.com/guybal/tarbac/api/v1"
	"github.com/guybal/tarbac/utils"
	"k8s.io/apimachinery/pkg/runtime"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/log"
	admission "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type SudoRequestAnnotator struct {
	Decoder admission.Decoder
	Scheme  *runtime.Scheme
}

func (a *SudoRequestAnnotator) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := log.FromContext(ctx)

	// Log incoming request details
	fmt.Printf("Incoming request details: Kind=%s, Name=%s, Namespace=%s, UserInfo=%+v\n", req.Kind.Kind, req.Name, req.Namespace, req.UserInfo)

	// Check if the decoder and scheme are set
	if a.Decoder == nil {
		utils.LogError(logger, nil, "Error: Decoder is not initialized")
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("decoder not initialized"))
	}
	if a.Scheme == nil {
		utils.LogError(logger, nil, "Error: Scheme is not initialized")
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("scheme not initialized"))
	}

	// Validate the incoming Kind
	switch req.Kind.Kind {
	case "SudoRequest":
		return a.handleSudoRequest(ctx, req)
	case "ClusterSudoRequest":
		return a.handleClusterSudoRequest(ctx, req)
	default:
		return admission.Denied("Ignoring unsupported resource kind")
	}
}

func (a *SudoRequestAnnotator) handleSudoRequest(ctx context.Context, req admission.Request) admission.Response {

	logger := log.FromContext(ctx)
	var sudoRequest v1.SudoRequest
	if err := a.Decoder.Decode(req, &sudoRequest); err != nil {
		utils.LogError(logger, err, fmt.Sprintf("Decode error for SudoRequest: %v\n", err))
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("failed to decode SudoRequest: %v", err))
	}

	fmt.Printf("Decoded SudoRequest: %+v\n", sudoRequest)

	// Add annotations
	if sudoRequest.Annotations == nil {
		sudoRequest.Annotations = map[string]string{}
	}

	if sudoRequest.Annotations["tarbac.io/requester"] == "" {
		sudoRequest.Annotations["tarbac.io/requester"] = req.UserInfo.Username
	} else if sudoRequest.Annotations["tarbac.io/requester"] != "" && sudoRequest.Annotations["tarbac.io/requester"] != req.UserInfo.Username {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("requesting user does not match the original requester"))
	}

	if sudoRequest.Annotations["tarbac.io/requester-metadata"] == "" {
		sudoRequest.Annotations["tarbac.io/requester-metadata"] = fmt.Sprintf("UID=%s, Groups=%v", req.UserInfo.UID, req.UserInfo.Groups)
	} else if sudoRequest.Annotations["tarbac.io/requester-metadata"] != "" && sudoRequest.Annotations["tarbac.io/requester-metadata"] != fmt.Sprintf("UID=%s, Groups=%v", req.UserInfo.UID, req.UserInfo.Groups) {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("requesting user does not match the original requester"))
	}

	utils.LogInfo(logger, fmt.Sprintf("Updated SudoRequest with annotations: %+v\n", sudoRequest.Annotations))

	return a.encodeAndPatchResponse(ctx, req, &sudoRequest)
}

func (a *SudoRequestAnnotator) handleClusterSudoRequest(ctx context.Context, req admission.Request) admission.Response {

	logger := log.FromContext(ctx)
	var clusterSudoRequest v1.ClusterSudoRequest
	if err := a.Decoder.Decode(req, &clusterSudoRequest); err != nil {
		utils.LogError(logger, err, fmt.Sprintf("Decode error for ClusterSudoRequest: %v\n", err))
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("failed to decode ClusterSudoRequest: %v", err))
	}

	utils.LogInfo(logger, fmt.Sprintf("Decoded ClusterSudoRequest: %+v\n", clusterSudoRequest))

	// Add annotations
	if clusterSudoRequest.Annotations == nil {
		clusterSudoRequest.Annotations = map[string]string{}
	}

	if clusterSudoRequest.Annotations["tarbac.io/requester"] == "" {
		clusterSudoRequest.Annotations["tarbac.io/requester"] = req.UserInfo.Username
	} else if clusterSudoRequest.Annotations["tarbac.io/requester"] != "" && clusterSudoRequest.Annotations["tarbac.io/requester"] != req.UserInfo.Username {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("requesting user does not match the original requester"))
	}

	if clusterSudoRequest.Annotations["tarbac.io/requester-metadata"] == "" {
		clusterSudoRequest.Annotations["tarbac.io/requester-metadata"] = fmt.Sprintf("UID=%s, Groups=%v", req.UserInfo.UID, req.UserInfo.Groups)
	} else if clusterSudoRequest.Annotations["tarbac.io/requester-metadata"] != "" && clusterSudoRequest.Annotations["tarbac.io/requester-metadata"] != fmt.Sprintf("UID=%s, Groups=%v", req.UserInfo.UID, req.UserInfo.Groups) {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("requesting user does not match the original requester"))
	}

	utils.LogInfo(logger, fmt.Sprintf("Updated ClusterSudoRequest with annotations: %+v\n", clusterSudoRequest.Annotations))

	return a.encodeAndPatchResponse(ctx, req, &clusterSudoRequest)
}

func (a *SudoRequestAnnotator) encodeAndPatchResponse(ctx context.Context, req admission.Request, obj runtime.Object) admission.Response {
	logger := log.FromContext(ctx)
	// Encode the mutated object using the manager's Scheme
	codecFactory := serializer.NewCodecFactory(a.Scheme)
	mutatedObject, err := runtime.Encode(codecFactory.LegacyCodec(v1.GroupVersion), obj)
	if err != nil {
		utils.LogError(logger, err, fmt.Sprintf("Encode error: %v\n", err))
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("failed to encode object: %v", err))
	}
	utils.LogInfo(logger, "Mutated object encoded successfully")

	// Return the mutation as a patch response
	return admission.PatchResponseFromRaw(req.Object.Raw, mutatedObject)
}

// InjectDecoder injects the decoder into the webhook
func (a *SudoRequestAnnotator) InjectDecoder(decoder admission.Decoder) error {
	if decoder == nil {
		fmt.Println("Error: Attempting to inject nil Decoder")
		return fmt.Errorf("cannot inject nil decoder")
	}
	a.Decoder = decoder
	fmt.Println("Decoder injected successfully")
	return nil
}

// InjectScheme injects the scheme into the webhook
func (a *SudoRequestAnnotator) InjectScheme(scheme *runtime.Scheme) error {
	if scheme == nil {
		fmt.Println("Error: Attempting to inject nil Scheme")
		return fmt.Errorf("cannot inject nil scheme")
	}
	a.Scheme = scheme
	fmt.Println("Scheme injected successfully")
	return nil
}
