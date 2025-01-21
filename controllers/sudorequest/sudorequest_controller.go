package controllers

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/guybal/tarbac/api/v1"
    utils "github.com/guybal/tarbac/utils"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
    "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"k8s.io/client-go/tools/record"
)

type SudoRequestReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Recorder record.EventRecorder // Add Recorder field
}

// Reconcile handles reconciliation for SudoRequest objects
func (r *SudoRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    logger := log.FromContext(ctx)
    var requestId string
    logger.Info("Reconciling SudoRequest", "name", req.Name, "namespace", req.Namespace)

    // Fetch the SudoRequest object
    var sudoRequest v1.SudoRequest
    if err := r.Get(ctx, req.NamespacedName, &sudoRequest); err != nil {
        if apierrors.IsNotFound(err) {
            logger.Info("SudoRequest resource not found. Ignoring since it must have been deleted.")
            return ctrl.Result{}, nil
        }
        logger.Error(err, "Unable to fetch SudoRequest")
        return ctrl.Result{}, err
    }

    if sudoRequest.Status.State == "Rejected" {
        logger.Info("SudoRequest rejected")
        return ctrl.Result{}, nil
    }

    if sudoRequest.Status.State == "Expired" {
        logger.Info("SudoRequest expired")
        return ctrl.Result{}, nil
    }

    duration, err := time.ParseDuration(sudoRequest.Spec.Duration)
    if err != nil {
        logger.Error(err, "Invalid duration in SudoRequest spec", "duration", sudoRequest.Spec.Duration)
        sudoRequest.Status.State = "Rejected"
        sudoRequest.Status.ErrorMessage = "Invalid requested duration"
        r.Recorder.Event(&sudoRequest, "Warning", "Rejected", fmt.Sprintf("SudoRequest from User %s for policy %s was rejected due to invalid requested duration [UID: %s]", sudoRequest.Annotations["tarbac.io/requester"], sudoRequest.Spec.Policy, sudoRequest.Status.RequestID))
        r.Status().Update(ctx, &sudoRequest)
        return ctrl.Result{}, err
    }

    requester := sudoRequest.Annotations["tarbac.io/requester"]

    // Check User
    if requester == "" {
        logger.Error(nil, "Requester annotation is missing")
        sudoRequest.Status.State = "Rejected"
        sudoRequest.Status.RequestID = string(sudoRequest.ObjectMeta.UID)
        sudoRequest.Status.ErrorMessage = "Requester information is missing"
        r.Recorder.Event(&sudoRequest, "Warning", "Rejected", fmt.Sprintf("SudoRequest from User %s for policy %s was rejected due to missing requester information [UID: %s]", sudoRequest.Annotations["tarbac.io/requester"], sudoRequest.Spec.Policy, sudoRequest.Status.RequestID))
        r.Status().Update(ctx, &sudoRequest)
        return ctrl.Result{}, fmt.Errorf("missing requester annotation")
    }

    // Initial State
    if sudoRequest.Status.State == "" {
        r.Recorder.Event(&sudoRequest, "Normal", "Submitted", fmt.Sprintf("User %s submitted a SudoRequest for policy %s for a duration of %s [UID: %s]", sudoRequest.Annotations["tarbac.io/requester"], sudoRequest.Spec.Policy, duration, string(sudoRequest.ObjectMeta.UID)))
        sudoRequest.Status.State = "Pending"
        requestId = string(sudoRequest.ObjectMeta.UID)
        sudoRequest.Status.RequestID = requestId

        if err := r.Client.Status().Update(ctx, &sudoRequest); err != nil {
            logger.Error(err, "Failed to set initial 'Pending' status")
            return ctrl.Result{}, err
        }

        if sudoRequest.ObjectMeta.Labels == nil {
               sudoRequest.ObjectMeta.Labels = make(map[string]string)
           }
           sudoRequest.ObjectMeta.Labels["tarbac.io/request-id"] = requestId
           // Update the object with the new label
           if err := r.Client.Update(ctx, &sudoRequest); err != nil {
               logger.Error(err, "Failed to update SudoRequest with RequestID label", "SudoRequest", sudoRequest.Name)
               return ctrl.Result{}, err
           }
    }

    // If the TemporaryRBAC is already created, fetch and update SudoRequest status
    if sudoRequest.Status.State == "Approved" {
        logger.Info("Fetching TemporaryRBAC for SudoRequest", "name", sudoRequest.Name)

        var temporaryRBAC v1.TemporaryRBAC
        err := r.Get(ctx, client.ObjectKey{
            Name:      utils.GenerateTempRBACName(rbacv1.Subject{Kind: "User", Name: requester}, sudoRequest.Spec.Policy, sudoRequest.Status.RequestID), // fmt.Sprintf("temporaryrbac-%s", sudoRequest.Name),
            Namespace: sudoRequest.Namespace,
        }, &temporaryRBAC)

        if err != nil {
            if apierrors.IsNotFound(err) {
                logger.Info("TemporaryRBAC not yet created, requeueing", "name", sudoRequest.Name)
                return ctrl.Result{RequeueAfter: time.Second * 5}, nil
            }
            logger.Error(err, "Failed to fetch TemporaryRBAC")
            sudoRequest.Status.State = "Error"
            sudoRequest.Status.ErrorMessage = "Failed to fetch TemporaryRBAC"
            r.Status().Update(ctx, &sudoRequest)
            return ctrl.Result{}, err
        }

        if temporaryRBAC.Status.State == "Expired" {
            sudoRequest.Status.State = "Expired"
            if err := r.Status().Update(ctx, &sudoRequest); err != nil {
                logger.Error(err, "Failed to update expired SudoRequest status")
                return ctrl.Result{}, err
            }
            r.Recorder.Event(&sudoRequest, "Warning", "Expired", fmt.Sprintf("SudoRequest Expired for User %s, revoked permissions for policy %s [UID: %s]", sudoRequest.Annotations["tarbac.io/requester"], sudoRequest.Spec.Policy, sudoRequest.Status.RequestID))
            logger.Info("SudoRequest has expired", "name", sudoRequest.Name)
            return ctrl.Result{}, nil
        }

        // Update SudoRequest status with TemporaryRBAC details
        sudoRequest.Status.ChildResource = []v1.ChildResource{
            {
                APIVersion:  temporaryRBAC.APIVersion,
                Kind:      "TemporaryRBAC",
                Name:      temporaryRBAC.Name,
                Namespace: temporaryRBAC.Namespace,
            },
        }

        if temporaryRBAC.Status.CreatedAt != nil && sudoRequest.Status.CreatedAt == nil {
            sudoRequest.Status.CreatedAt = temporaryRBAC.Status.CreatedAt
        }
        if temporaryRBAC.Status.ExpiresAt != nil && sudoRequest.Status.ExpiresAt == nil {
            sudoRequest.Status.ExpiresAt = temporaryRBAC.Status.ExpiresAt
        }
//         sudoRequest.Status.State = temporaryRBAC.Status.State
        if err := r.Status().Update(ctx, &sudoRequest); err != nil {
            logger.Error(err, "Failed to update SudoRequest status with TemporaryRBAC details")
            return ctrl.Result{}, err
        }

        logger.Info("Successfully updated SudoRequest status with TemporaryRBAC details", "TemporaryRBAC", temporaryRBAC.Name)

        // Requeue until expiration
        timeUntilExpiration := time.Until(sudoRequest.Status.ExpiresAt.Time)

        if timeUntilExpiration <= 0 {
            logger.Info("SudoRequest has expired",
                "expiredBy", -timeUntilExpiration, // Log how long ago it expired
                "name", sudoRequest.Name,
                "namespace", sudoRequest.Namespace)

            sudoRequest.Status.State = "Expired"
            if err := r.Status().Update(ctx, &sudoRequest); err != nil {
                logger.Error(err, "Failed to update expired SudoRequest status")
                return ctrl.Result{}, err
            }
            r.Recorder.Event(&sudoRequest, "Warning", "Expired", fmt.Sprintf("SudoRequest Expired for User %s, revoked permissions for policy %s [UID: %s]", sudoRequest.Annotations["tarbac.io/requester"], sudoRequest.Spec.Policy, sudoRequest.Status.RequestID))
            return ctrl.Result{}, nil
        }

        logger.Info("Requeueing for expiration check", "timeUntilExpiration", timeUntilExpiration)
        return ctrl.Result{RequeueAfter: timeUntilExpiration}, nil
    }

    // If TemporaryRBAC is not yet created, create it
    if sudoRequest.Status.State == "Pending" {
        logger.Info("Creating TemporaryRBAC for SudoRequest", "name", sudoRequest.Name)

        var sudoPolicy v1.SudoPolicy
        if err := r.Get(ctx, client.ObjectKey{
            Name:      sudoRequest.Spec.Policy,
            Namespace: sudoRequest.Namespace,
        }, &sudoPolicy); err != nil {
            logger.Error(err, "Failed to fetch referenced SudoPolicy")
            sudoRequest.Status.State = "Rejected"
            sudoRequest.Status.ErrorMessage = "Referenced policy not found"
            r.Recorder.Event(&sudoRequest, "Warning", "Rejected", fmt.Sprintf("SudoRequest for User %s was rejected due to missing policy: %s [UID: %s]", sudoRequest.Annotations["tarbac.io/requester"], sudoRequest.Spec.Policy, sudoRequest.Status.RequestID))
            r.Status().Update(ctx, &sudoRequest)
            return ctrl.Result{}, err
        }

        maxDuration, err := time.ParseDuration(sudoPolicy.Spec.MaxDuration)
        if err != nil {
            logger.Error(err, "Invalid maxDuration in SudoPolicy spec", "maxDuration", sudoPolicy.Spec.MaxDuration)
            sudoRequest.Status.State = "Rejected"
            sudoRequest.Status.ErrorMessage = "Invalid policy max duration"
            r.Status().Update(ctx, &sudoRequest)
            return ctrl.Result{}, err
        }

        if duration > maxDuration || duration <=0 {
            logger.Info("SudoRequest duration exceeds maxDuration in policy",
                "requestedDuration", duration, "maxDuration", maxDuration)
            sudoRequest.Status.State = "Rejected"
            sudoRequest.Status.ErrorMessage = fmt.Sprintf("Requested duration %s exceeds max allowed duration %s", duration, maxDuration)
            r.Recorder.Event(&sudoRequest, "Warning", "Rejected",
                fmt.Sprintf("SudoRequest from User %s was rejected: requested duration %s exceeds max allowed duration %s [UID: %s]",
                    sudoRequest.Annotations["tarbac.io/requester"], duration, maxDuration, sudoRequest.Status.RequestID))
            r.Status().Update(ctx, &sudoRequest)
            return ctrl.Result{}, nil
        }
        logger.Info("SudoRequest duration validated against SudoPolicy", "requestedDuration", duration, "maxDuration", maxDuration)

        // Check if the user is allowed
        allowed := false
        for _, user := range sudoPolicy.Spec.AllowedUsers {
            if user.Name == requester {
                allowed = true
                logger.Info("User approved by policy", "user", requester, "policy", sudoPolicy.Name)
                break
            }
        }

        if !allowed {
            logger.Info("SudoRequest rejected: User not allowed by policy",
                "user", sudoRequest.Annotations["tarbac.io/requester"],
                "policy", sudoRequest.Spec.Policy,
                "requestID", sudoRequest.Status.RequestID)
            sudoRequest.Status.State = "Rejected"
            sudoRequest.Status.ErrorMessage = "User not allowed by policy"
            r.Recorder.Event(&sudoRequest, "Warning", "Rejected",
                fmt.Sprintf("SudoRequest from User %s was rejected: User not allowed by policy %s [UID: %s]",
                    sudoRequest.Annotations["tarbac.io/requester"], sudoRequest.Spec.Policy, sudoRequest.Status.RequestID))
            r.Status().Update(ctx, &sudoRequest)
//             logger.Info("SudoRequest rejected", "user", requester, "policy", sudoPolicy.Name)
            return ctrl.Result{}, nil
        }

        temporaryRBAC := &v1.TemporaryRBAC{
            ObjectMeta: metav1.ObjectMeta{
                Name:      utils.GenerateTempRBACName(rbacv1.Subject{Kind: "User", Name: requester}, sudoRequest.Spec.Policy, sudoRequest.Status.RequestID), // fmt.Sprintf("temporaryrbac-%s", sudoRequest.Name),
                Namespace: sudoRequest.Namespace,
            },
            Spec: v1.TemporaryRBACSpec{
                Subjects: []rbacv1.Subject{
                    {
                        Kind: "User",
                        Name: requester,
                    },
                },
                RoleRef: rbacv1.RoleRef{
                    APIGroup: sudoPolicy.Spec.RoleRef.APIGroup,
                    Kind:     sudoPolicy.Spec.RoleRef.Kind,
                    Name:     sudoPolicy.Spec.RoleRef.Name,
                },
                Duration: sudoRequest.Spec.Duration,
            },
        }

        if err := controllerutil.SetControllerReference(&sudoRequest, temporaryRBAC, r.Scheme); err != nil {
            logger.Error(err, "Failed to set OwnerReference on TemporaryRBAC")
            return ctrl.Result{}, err
        }

        if err := r.Create(ctx, temporaryRBAC); err != nil {
            logger.Error(err, "Failed to create TemporaryRBAC")
            sudoRequest.Status.State = "Error"
            sudoRequest.Status.ErrorMessage = "Failed to create TemporaryRBAC"
            r.Status().Update(ctx, &sudoRequest)
            return ctrl.Result{}, err
        }

        // Update the SudoRequest status to Approved
        sudoRequest.Status.State = "Approved"
        if temporaryRBAC.Status.CreatedAt != nil && sudoRequest.Status.CreatedAt == nil {
            sudoRequest.Status.CreatedAt = temporaryRBAC.Status.CreatedAt
        }
        if temporaryRBAC.Status.ExpiresAt != nil && sudoRequest.Status.ExpiresAt == nil {
            sudoRequest.Status.ExpiresAt = temporaryRBAC.Status.ExpiresAt
        }

        if err := r.Status().Update(ctx, &sudoRequest); err != nil {
            logger.Error(err, "Failed to update SudoRequest status to Approved")
            return ctrl.Result{}, err
        }

		r.Recorder.Event(&sudoRequest, "Normal", "Approved", fmt.Sprintf("TemporaryRBAC created for User: %s using %s SudoPolicy [UID: %s]", sudoRequest.Annotations["tarbac.io/requester"], sudoRequest.Spec.Policy, sudoRequest.Status.RequestID))
        logger.Info("TemporaryRBAC created and SudoRequest status updated to Approved", "TemporaryRBAC", temporaryRBAC.Name)
        return ctrl.Result{RequeueAfter: time.Second * 5}, nil
    }

    return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SudoRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
    r.Scheme = mgr.GetScheme() // Initialize the Scheme field
    r.Recorder = mgr.GetEventRecorderFor("SudoRequestController") // Properly initialize Recorder
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.SudoRequest{}).
		Complete(r)
}
