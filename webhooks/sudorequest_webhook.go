package webhooks

import (
	"context"
	"fmt"
	"net/http"

	v1 "github.com/guybal/tarbac/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer"
	admission "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type SudoRequestAnnotator struct {
	Decoder admission.Decoder
	Scheme  *runtime.Scheme
}

func (a *SudoRequestAnnotator) Handle(ctx context.Context, req admission.Request) admission.Response {
	// Log incoming request details
	fmt.Printf("Incoming request details: Kind=%s, Name=%s, Namespace=%s, UserInfo=%+v\n", req.Kind.Kind, req.Name, req.Namespace, req.UserInfo)

	// Check if the decoder and scheme are set
	if a.Decoder == nil {
		fmt.Println("Error: Decoder is not initialized")
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("decoder not initialized"))
	}
	if a.Scheme == nil {
		fmt.Println("Error: Scheme is not initialized")
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
	var sudoRequest v1.SudoRequest
	if err := a.Decoder.Decode(req, &sudoRequest); err != nil {
		fmt.Printf("Decode error for SudoRequest: %v\n", err)
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("failed to decode SudoRequest: %v", err))
	}

	fmt.Printf("Decoded SudoRequest: %+v\n", sudoRequest)

	// Add annotations
	if sudoRequest.Annotations == nil {
		sudoRequest.Annotations = map[string]string{}
	}
	if sudoRequest.Annotations["tarbac.io/requester"] == "" {
	    sudoRequest.Annotations["tarbac.io/requester"] = req.UserInfo.Username
	}
	if sudoRequest.Annotations["tarbac.io/requester-metadata"] == "" {
        sudoRequest.Annotations["tarbac.io/requester-metadata"] = fmt.Sprintf("UID=%s, Groups=%v", req.UserInfo.UID, req.UserInfo.Groups)
	}

	fmt.Printf("Updated SudoRequest with annotations: %+v\n", sudoRequest.Annotations)

	return a.encodeAndPatchResponse(req, &sudoRequest)
}

func (a *SudoRequestAnnotator) handleClusterSudoRequest(ctx context.Context, req admission.Request) admission.Response {
	var clusterSudoRequest v1.ClusterSudoRequest
	if err := a.Decoder.Decode(req, &clusterSudoRequest); err != nil {
		fmt.Printf("Decode error for ClusterSudoRequest: %v\n", err)
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("failed to decode ClusterSudoRequest: %v", err))
	}

	fmt.Printf("Decoded ClusterSudoRequest: %+v\n", clusterSudoRequest)

	// Add annotations
	if clusterSudoRequest.Annotations == nil {
		clusterSudoRequest.Annotations = map[string]string{}
	}
	if clusterSudoRequest.Annotations["tarbac.io/requester"] == "" {
        clusterSudoRequest.Annotations["tarbac.io/requester"] = req.UserInfo.Username
    }
    if clusterSudoRequest.Annotations["tarbac.io/requester-metadata"] == "" {
        clusterSudoRequest.Annotations["tarbac.io/requester-metadata"] = fmt.Sprintf("UID=%s, Groups=%v", req.UserInfo.UID, req.UserInfo.Groups)
    }

	fmt.Printf("Updated ClusterSudoRequest with annotations: %+v\n", clusterSudoRequest.Annotations)

	return a.encodeAndPatchResponse(req, &clusterSudoRequest)
}

func (a *SudoRequestAnnotator) encodeAndPatchResponse(req admission.Request, obj runtime.Object) admission.Response {
	// Encode the mutated object using the manager's Scheme
	codecFactory := serializer.NewCodecFactory(a.Scheme)
	mutatedObject, err := runtime.Encode(codecFactory.LegacyCodec(v1.GroupVersion), obj)
	if err != nil {
		fmt.Printf("Encode error: %v\n", err)
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("failed to encode object: %v", err))
	}
	fmt.Printf("Mutated object encoded successfully\n")

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


// package webhooks
//
// import (
// 	"context"
// 	"fmt"
// 	"net/http"
//
// 	v1 "github.com/guybal/tarbac/api/v1"
//     "k8s.io/apimachinery/pkg/runtime"
//     serializer "k8s.io/apimachinery/pkg/runtime/serializer"
//     admission "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
// )
//
// type SudoRequestAnnotator struct {
// 	Decoder admission.Decoder
// 	Scheme  *runtime.Scheme
// }
//
// func (a *SudoRequestAnnotator) Handle(ctx context.Context, req admission.Request) admission.Response {
//
// 	// Log incoming request details
//     fmt.Printf("Incoming request details: Kind=%s, Name=%s, Namespace=%s, UserInfo=%+v\n", req.Kind.Kind, req.Name, req.Namespace, req.UserInfo)
//
//     // Check if the decoder and scheme are set
//     if a.Decoder == nil {
//         fmt.Println("Error: Decoder is not initialized")
//         return admission.Errored(http.StatusInternalServerError, fmt.Errorf("decoder not initialized"))
//     }
//     if a.Scheme == nil {
//         fmt.Println("Error: Scheme is not initialized")
//         return admission.Errored(http.StatusInternalServerError, fmt.Errorf("scheme not initialized"))
//     }
//
// 	// Validate the incoming Kind
// 	if req.Kind.Kind != "SudoRequest" && req.Kind.Kind != "ClusterSudoRequest" {
// 		return admission.Allowed("Ignoring non-SudoRequest or non-ClusterSudoRequest resource")
// 	}
//
// 	// Log the raw object for inspection
//     if req.Object.Raw == nil {
//         return admission.Errored(http.StatusBadRequest, fmt.Errorf("request object is nil"))
//     }
//     fmt.Printf("Raw request object: %s\n", string(req.Object.Raw))
//
// 	// Decode the incoming object
// 	var sudoRequest v1.SudoRequest
// 	if err := a.Decoder.Decode(req, &sudoRequest); err != nil {
//         fmt.Printf("Decode error: %v\n", err)
// 		return admission.Errored(http.StatusBadRequest, fmt.Errorf("failed to decode SudoRequest: %v", err))
// 	}
//     fmt.Printf("Decoded SudoRequest: %+v\n", sudoRequest)
//
// 	// Add annotation
// 	if sudoRequest.Annotations == nil {
// 		sudoRequest.Annotations = map[string]string{}
// 	}
// 	sudoRequest.Annotations["tarbac.io/requester"] = req.UserInfo.Username
//     sudoRequest.Annotations["tarbac.io/requester-metadata"] = fmt.Sprintf("UID=%s, Groups=%v", req.UserInfo.UID, req.UserInfo.Groups)
//
//     fmt.Printf("Updated SudoRequest with annotations: %+v\n", sudoRequest.Annotations)
//
// 	// Encode the mutated object using the manager's Scheme
// 	codecFactory := serializer.NewCodecFactory(a.Scheme)
// 	mutatedObject, err := runtime.Encode(
// 		codecFactory.LegacyCodec(v1.GroupVersion),
// 		&sudoRequest,
// 	)
// 	if err != nil {
//         fmt.Printf("Encode error: %v\n", err)
// 		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("failed to encode object: %v", err))
// 	}
//     fmt.Printf("Mutated object encoded successfully\n")
//
// 	// Return the mutation as a patch response
// 	return admission.PatchResponseFromRaw(req.Object.Raw, mutatedObject)
// }
//
// // InjectDecoder injects the decoder into the webhook
// func (a *SudoRequestAnnotator) InjectDecoder(decoder admission.Decoder) error {
// 	if decoder == nil {
// 		fmt.Println("Error: Attempting to inject nil Decoder")
// 		return fmt.Errorf("cannot inject nil decoder")
// 	}
// 	a.Decoder = decoder
// 	fmt.Println("Decoder injected successfully")
// 	return nil
// }
//
// // InjectScheme injects the scheme into the webhook
// func (a *SudoRequestAnnotator) InjectScheme(scheme *runtime.Scheme) error {
// 	if scheme == nil {
// 		fmt.Println("Error: Attempting to inject nil Scheme")
// 		return fmt.Errorf("cannot inject nil scheme")
// 	}
// 	a.Scheme = scheme
// 	fmt.Println("Scheme injected successfully")
// 	return nil
// }
