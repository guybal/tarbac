package controllers

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/guybal/tarbac/api/v1"
	utils "github.com/guybal/tarbac/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type SudoPolicyReconciler struct {
	client.Client
	Recorder record.EventRecorder
}

// Reconcile handles reconciliation for SudoPolicy objects
func (r *SudoPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	utils.LogInfo(logger, "Reconciling SudoPolicy", "name", req.Name)

	// Fetch the SudoPolicy object
	var sudoPolicy v1.SudoPolicy
	if err := r.Get(ctx, req.NamespacedName, &sudoPolicy); err != nil {
		if apierrors.IsNotFound(err) {
			utils.LogInfo(logger, "SudoPolicy resource not found. Ignoring since it must have been deleted.", "name", req.Name, "namespace", req.Namespace)
			return ctrl.Result{}, nil
		}
		utils.LogError(logger, err, "Unable to fetch SudoPolicy", "name", req.Name, "namespace", req.Namespace)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Validate maxDuration
	if _, err := time.ParseDuration(sudoPolicy.Spec.MaxDuration); err != nil {
		return r.errorRequest(ctx, err, &sudoPolicy, fmt.Sprintf("Invalid MaxDuration in SudoPolicy spec: %s", sudoPolicy.Spec.MaxDuration))
	}

	// Update SudoPolicy status
	sudoPolicy.Status.State = "Active"
	if err := r.Status().Update(ctx, &sudoPolicy); err != nil {
		return r.errorRequest(ctx, err, &sudoPolicy, "Failed to update SudoPolicy status")
	}

	utils.LogInfo(logger, "Successfully validated SudoPolicy", "name", sudoPolicy.Name, "kind", sudoPolicy.Kind, "namespace", sudoPolicy.Namespace)
	return ctrl.Result{}, nil
}

func (r *SudoPolicyReconciler) errorRequest(ctx context.Context, err error, sudoPolicy *v1.SudoPolicy, message string) (ctrl.Result, error) {

	logger := log.FromContext(ctx)
	utils.LogError(logger, err, "SudoPolicy Error", "errorMessage", message)
	sudoPolicy.Status.State = "Error"
	sudoPolicy.Status.ErrorMessage = message
	if err := r.Status().Update(ctx, sudoPolicy); err != nil {
		utils.LogError(logger, err, "Failed to update SudoPolicy status to Error")
		return ctrl.Result{}, err
	}
	// r.Recorder.Event(sudoPolicy, "Error", "SudoPolicyError", message)
	eventMessage := fmt.Sprintf("SudoPolicyError in policy '%s': %s", sudoPolicy.Name, message)
	r.Recorder.Event(sudoPolicy, "Error", "SudoPolicyError", eventMessage)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SudoPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.SudoPolicy{}).
		Complete(r)
}
