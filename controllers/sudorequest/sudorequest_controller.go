package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	v1 "github.com/guybal/tarbac/api/v1"
	utils "github.com/guybal/tarbac/utils"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type SudoRequestReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// Reconcile handles reconciliation for SudoRequest objects
func (r *SudoRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	var requestId string

	utils.LogInfo(logger, "Reconciling SudoRequest", "name", req.Name)

	// Fetch the SudoRequest object
	var sudoRequest v1.SudoRequest
	if err := r.Get(ctx, req.NamespacedName, &sudoRequest); err != nil {
		if apierrors.IsNotFound(err) {
			utils.LogInfo(logger, "SudoRequest resource not found. Ignoring since it must have been deleted.", "name", req.Name)
			return ctrl.Result{}, nil
		}
		utils.LogError(logger, err, "Unable to fetch SudoRequest", "name", req.Name)
		return ctrl.Result{}, err
	}

	requestId = r.getRequestID(&sudoRequest)

	if sudoRequest.Status.State == "Rejected" || sudoRequest.Status.State == "Expired" {
		utils.LogInfoUID(logger, "SudoRequest already processed", requestId, "state", sudoRequest.Status.State)
		return ctrl.Result{}, nil
	}

	// Validate duration
	duration, err := time.ParseDuration(sudoRequest.Spec.Duration)
	if err != nil || duration <= 0 {
		return r.rejectRequest(ctx, &sudoRequest, fmt.Sprintf("Invalid duration requested: %s", sudoRequest.Spec.Duration), requestId)
	}

	// Validate requester
	requester := sudoRequest.Annotations["tarbac.io/requester"]
	if requester == "" {
		return r.rejectRequest(ctx, &sudoRequest, "Requester information is missing", requestId)
	}

	// Validate referenced policy exists
	var sudoPolicy v1.SudoPolicy
	if err := r.Get(ctx, client.ObjectKey{Name: sudoRequest.Spec.Policy, Namespace: sudoRequest.Namespace}, &sudoPolicy); err != nil {
		return r.rejectRequest(ctx, &sudoRequest, "Referenced policy not found", requestId)
	}

	// Initial State
	if sudoRequest.Status.State == "" {
		r.Recorder.Event(&sudoRequest, "Normal", "Submitted", fmt.Sprintf("User %s submitted a SudoRequest for policy %s for a duration of %s [UID: %s]", sudoRequest.Annotations["tarbac.io/requester"], sudoRequest.Spec.Policy, duration, string(sudoRequest.ObjectMeta.UID)))
		sudoRequest.Status.State = "Pending"
		sudoRequest.Status.RequestID = requestId

		if err := r.Client.Status().Update(ctx, &sudoRequest); err != nil {
			utils.LogErrorUID(logger, err, "Failed to set initial 'Pending' status", requestId, "SudoRequest", sudoRequest.Name)
			return ctrl.Result{}, err
		}

		if sudoRequest.ObjectMeta.Labels == nil {
			sudoRequest.ObjectMeta.Labels = make(map[string]string)
		}
		sudoRequest.ObjectMeta.Labels["tarbac.io/request-id"] = requestId
		// Update the object with the new label
		if err := r.Client.Update(ctx, &sudoRequest); err != nil {
			return r.errorRequest(ctx, err, &sudoRequest, "Failed to update SudoRequest with RequestID label", requestId)

		}
	}

	// If TemporaryRBAC is not yet created, create it
	if sudoRequest.Status.State == "Pending" {

		maxDuration, err := time.ParseDuration(sudoPolicy.Spec.MaxDuration)
		if err != nil {
			return r.errorRequest(ctx, err, &sudoRequest, "Invalid maxDuration in SudoPolicy spec", requestId)
		}

		if duration > maxDuration {
			return r.rejectRequest(ctx, &sudoRequest, fmt.Sprintf("Requested duration %s exceeds max allowed duration %s", duration, maxDuration), requestId)
		}

		if !r.validateRequester(sudoPolicy, requester) {
			return r.rejectRequest(ctx, &sudoRequest, "User not allowed by policy", requestId)
		}

		namespaces := []string{sudoRequest.Namespace}

		r.Recorder.Event(&sudoRequest, "Normal", "Approved", fmt.Sprintf("User '%s' was approved by '%s' SudoPolicy [UID: %s]", requester, sudoPolicy.Name, requestId))
		return r.createTemporaryRBACsForNamespaces(ctx, &sudoRequest, namespaces, &sudoPolicy, requester, logger, requestId)
	}

	// If the TemporaryRBAC is already created, fetch and update SudoRequest status
	if sudoRequest.Status.State == "Approved" {

		utils.LogInfoUID(logger, "SudoRequest is already approved, validating child resource", requestId)

		for _, childResource := range sudoRequest.Status.ChildResource {

			if childResource.Name == "" {
				utils.LogErrorUID(logger, nil, "Child resource has incomplete data", requestId, "childResource", childResource)
				continue
			}

			// Fetch child resource
			switch childResource.Kind {
			case "TemporaryRBAC":
				var temporaryRBAC v1.TemporaryRBAC
				err := r.Get(ctx, client.ObjectKey{Name: childResource.Name, Namespace: childResource.Namespace}, &temporaryRBAC)
				if err != nil {
					if apierrors.IsNotFound(err) {
						utils.LogErrorUID(logger, err, "Child TemporaryRBAC resource not found", requestId, "child", childResource)
						r.Recorder.Event(&sudoRequest, "Warning", "MissingChildResource", fmt.Sprintf("Child resource %s/%s not found", childResource.Namespace, childResource.Name))
						continue
					}
					utils.LogErrorUID(logger, err, "Failed to fetch child resource", requestId, "child", childResource)
					continue
				}

				if temporaryRBAC.Status.CreatedAt != nil && sudoRequest.Status.CreatedAt == nil {
					sudoRequest.Status.CreatedAt = temporaryRBAC.Status.CreatedAt
				}
				if temporaryRBAC.Status.ExpiresAt != nil && sudoRequest.Status.ExpiresAt == nil {
					sudoRequest.Status.ExpiresAt = temporaryRBAC.Status.ExpiresAt
				}

				// Check the state of the child resource
				switch temporaryRBAC.Status.State {
				case "Expired":
					sudoRequest.Status.State = "Expired"
					if err := r.Status().Update(ctx, &sudoRequest); err != nil {
						return r.errorRequest(ctx, err, &sudoRequest, "Failed to update expired SudoRequest status", requestId)
					}
					r.Recorder.Event(&sudoRequest, "Warning", "Expired", fmt.Sprintf("SudoRequest Expired for User %s, revoked permissions for policy %s [UID: %s]", requester, sudoRequest.Spec.Policy, requestId))
					utils.LogInfoUID(logger, "SudoRequest has expired", requestId, "name", sudoRequest.Name)
					return ctrl.Result{}, nil
				case "Error":
					sudoRequest.Status.State = "Error"
					if err := r.Status().Update(ctx, &sudoRequest); err != nil {
						return r.errorRequest(ctx, err, &sudoRequest, "Failed to update expired SudoRequest status", requestId)
					}
					r.Recorder.Event(&sudoRequest, "Error", "Error", fmt.Sprintf("Error detected while processing SudoRequest for User '%s' and policy '%s' [UID: %s]", requester, sudoRequest.Spec.Policy, requestId))
					utils.LogInfoUID(logger, "SudoRequest has errors", requestId, "name", sudoRequest.Name)
					return ctrl.Result{}, nil
				}
			}
		}

		// Update the SudoRequest status
		if err := r.Client.Status().Update(ctx, &sudoRequest); err != nil {
			return r.errorRequest(ctx, err, &sudoRequest, "Failed to update SudoRequest status after child resource validation", requestId)
		}

		utils.LogInfoUID(logger, "ClusterSudoRequest status updated based on child resources", requestId, "state", sudoRequest.Status.State)
	}
	return ctrl.Result{}, nil
}

func (r *SudoRequestReconciler) validateRequester(policy v1.SudoPolicy, requester string) bool {
	for _, user := range policy.Spec.AllowedUsers {
		if user.Name == requester {
			return true
		}
	}
	return false
}

func (r *SudoRequestReconciler) rejectRequest(ctx context.Context, sudoRequest *v1.SudoRequest, message string, requestID string) (ctrl.Result, error) {

	logger := log.FromContext(ctx)
	utils.LogInfoUID(logger, "Rejecting SudoRequest", requestID, "errorMessage", message)
	sudoRequest.Status.State = "Rejected"
	sudoRequest.Status.ErrorMessage = message
	if err := r.Status().Update(ctx, sudoRequest); err != nil {
		utils.LogErrorUID(logger, err, "Failed to update SudoRequest status to Rejected", requestID)
		return ctrl.Result{}, err
	}
	r.Recorder.Event(sudoRequest, "Warning", "Rejected", fmt.Sprintf("%s [UID: %s]", message, requestID))
	return ctrl.Result{}, nil
}

func (r *SudoRequestReconciler) errorRequest(ctx context.Context, err error, sudoRequest *v1.SudoRequest, message string, requestID string) (ctrl.Result, error) {

	logger := log.FromContext(ctx)
	utils.LogErrorUID(logger, err, "SudoRequest Error", requestID, "errorMessage", message)
	sudoRequest.Status.State = "Error"
	sudoRequest.Status.ErrorMessage = message
	if err := r.Status().Update(ctx, sudoRequest); err != nil {
		utils.LogErrorUID(logger, err, "Failed to update SudoRequest status to Error", requestID)
		return ctrl.Result{}, err
	}
	r.Recorder.Event(sudoRequest, "Error", "SudoRequestError", fmt.Sprintf("%s [UID: %s]", message, requestID))
	return ctrl.Result{}, nil
}

func (r *SudoRequestReconciler) getRequestID(sudoRequest *v1.SudoRequest) string {
	var requestId string
	if sudoRequest.Status.RequestID != "" {
		requestId = sudoRequest.Status.RequestID
	} else {
		requestId = string(sudoRequest.ObjectMeta.UID)
	}
	return requestId
}

func (r *SudoRequestReconciler) createTemporaryRBACsForNamespaces(ctx context.Context, sudoRequest *v1.SudoRequest, namespaces []string, sudoPolicy *v1.SudoPolicy, requester string, logger logr.Logger, requestId string) (ctrl.Result, error) {
	var childResources []v1.ChildResource

	for _, namespace := range namespaces {
		temporaryRBAC := &v1.TemporaryRBAC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      utils.GenerateTempRBACName(rbacv1.Subject{Kind: "User", Name: requester}, sudoRequest.Spec.Policy, sudoRequest.Status.RequestID),
				Namespace: namespace,
			},
			Spec: v1.TemporaryRBACSpec{
				Subjects: []rbacv1.Subject{
					{
						Kind: "User",
						Name: requester,
					},
				},
				RoleRef:  sudoPolicy.Spec.RoleRef,
				Duration: sudoRequest.Spec.Duration,
			},
		}

		if err := controllerutil.SetControllerReference(sudoRequest, temporaryRBAC, r.Scheme); err != nil {
			utils.LogErrorUID(logger, err, "Failed to set OwnerReference on TemporaryRBAC", requestId)
			continue
		}

		if err := r.Client.Create(ctx, temporaryRBAC); err != nil {
			utils.LogErrorUID(logger, err, "Failed to create TemporaryRBAC", requestId)
			continue
		}

		utils.LogInfoUID(logger, "TemporaryRBAC created successfully", requestId, "temporaryRBAC", temporaryRBAC.Name, "namespace", temporaryRBAC.Namespace)

		childResources = append(childResources, v1.ChildResource{
			APIVersion: "tarbac.io/v1",
			Kind:       "TemporaryRBAC",
			Name:       temporaryRBAC.Name,
			Namespace:  namespace,
		})
	}

	sudoRequest.Status.State = "Approved"
	sudoRequest.Status.ChildResource = childResources

	if err := r.Status().Update(ctx, sudoRequest); err != nil {
		utils.LogErrorUID(logger, err, "Failed to update SudoRequest status with TemporaryRBAC details", requestId)
		return ctrl.Result{}, err
	}
	utils.LogInfoUID(logger, "Successfully updated SudoRequest status with TemporaryRBAC details, requeuing while waiting for child resource creation", requestId)
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SudoRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Scheme = mgr.GetScheme()                                    // Initialize the Scheme field
	r.Recorder = mgr.GetEventRecorderFor("SudoRequestController") // Properly initialize Recorder
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.SudoRequest{}).
		Complete(r)
}
