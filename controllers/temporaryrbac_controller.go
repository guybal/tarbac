package controllers

import (
	"context"
	"time"

	"github.com/guybal/tarbac/api/v1" // Ensure this matches your module path
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TemporaryRBACReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *TemporaryRBACReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Fetch TemporaryRBAC resource
	var tempRBAC v1.TemporaryRBAC
	if err := r.Get(ctx, req.NamespacedName, &tempRBAC); err != nil {
		// Ignore not-found errors (e.g., when the resource has been deleted)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Parse the duration from the CRD
	duration, err := time.ParseDuration(tempRBAC.Spec.Duration)
	if err != nil {
		// Log and update status if the duration is invalid
		return ctrl.Result{}, err
	}

	// Calculate expiration time
	expiration := tempRBAC.CreationTimestamp.Add(duration)

	// If expired, clean up RoleBinding or ClusterRoleBinding
	if time.Now().After(expiration) {
		if err := r.cleanupBindings(ctx, &tempRBAC); err != nil {
			return ctrl.Result{}, err
		}
		// Delete the TemporaryRBAC resource itself
		if err := r.Delete(ctx, &tempRBAC); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Ensure RoleBinding or ClusterRoleBinding exists
	if err := r.ensureBindings(ctx, &tempRBAC); err != nil {
		return ctrl.Result{}, err
	}

	// Requeue the resource for expiration
	return ctrl.Result{RequeueAfter: time.Until(expiration)}, nil
}

func (r *TemporaryRBACReconciler) ensureBindings(ctx context.Context, tempRBAC *v1.TemporaryRBAC) error {
	bindingName := tempRBAC.Name + "-binding"

	// Create RoleBinding or ClusterRoleBinding based on the RoleRef type
	var binding client.Object
	if tempRBAC.Spec.RoleRef.Kind == "ClusterRole" {
		binding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: bindingName,
			},
			Subjects: []rbacv1.Subject{tempRBAC.Spec.Subject},
			RoleRef:  tempRBAC.Spec.RoleRef,
		}
	} else {
		binding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      bindingName,
				Namespace: tempRBAC.Namespace,
			},
			Subjects: []rbacv1.Subject{tempRBAC.Spec.Subject},
			RoleRef:  tempRBAC.Spec.RoleRef,
		}
	}

	// Create or update the binding
	return r.Client.Create(ctx, binding)
}

func (r *TemporaryRBACReconciler) cleanupBindings(ctx context.Context, tempRBAC *v1.TemporaryRBAC) error {
	bindingName := tempRBAC.Name + "-binding"

	// Delete RoleBinding or ClusterRoleBinding
	if tempRBAC.Spec.RoleRef.Kind == "ClusterRole" {
		return r.Client.Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: bindingName}})
	}
	return r.Client.Delete(ctx, &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: bindingName, Namespace: tempRBAC.Namespace}})
}

func (r *TemporaryRBACReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.TemporaryRBAC{}).
		Complete(r)
}
