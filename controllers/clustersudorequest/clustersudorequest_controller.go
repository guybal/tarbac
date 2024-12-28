package controllers

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/guybal/tarbac/api/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"github.com/go-logr/logr"
)

type ClusterSudoRequestReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

func (r *ClusterSudoRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
    var requestId string
	logger.Info("Reconciling ClusterSudoRequest", "name", req.Name, "namespace", req.Namespace)

	var clusterSudoRequest v1.ClusterSudoRequest
	if err := r.Get(ctx, req.NamespacedName, &clusterSudoRequest); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("ClusterSudoRequest resource not found. Ignoring since it must have been deleted.")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Unable to fetch ClusterSudoRequest")
		return ctrl.Result{}, err
	}

    duration, err := time.ParseDuration(clusterSudoRequest.Spec.Duration)
    if err != nil || duration <= 0 {
        return r.rejectRequest(ctx, &clusterSudoRequest, fmt.Sprintf("Invalid duration requested: %s", clusterSudoRequest.Spec.Duration), logger)
    }

    requester := clusterSudoRequest.Annotations["tarbac.io/requester"]
    if requester == "" {
        return r.rejectRequest(ctx, &clusterSudoRequest, "Requester information is missing", logger)
    }

    var clusterSudoPolicy v1.ClusterSudoPolicy
    if err := r.Get(ctx, client.ObjectKey{Name: clusterSudoRequest.Spec.Policy}, &clusterSudoPolicy); err != nil {
        return r.rejectRequest(ctx, &clusterSudoRequest, "Referenced policy not found", logger)
    }

	// Initial State
    if clusterSudoRequest.Status.State == "" {
        r.Recorder.Event(&clusterSudoRequest, "Normal", "Submitted", fmt.Sprintf("User %s submitted a ClusterSudoRequest for policy %s for a duration of %s [UID: %s]", clusterSudoRequest.Annotations["tarbac.io/requester"], clusterSudoRequest.Spec.Policy, duration, string(clusterSudoRequest.ObjectMeta.UID)))
        clusterSudoRequest.Status.State = "Pending"
        requestId = string(clusterSudoRequest.ObjectMeta.UID)
        clusterSudoRequest.Status.RequestID = requestId
        if err := r.Client.Status().Update(ctx, &clusterSudoRequest); err != nil {
            logger.Error(err, "Failed to set initial 'Pending' status")
            return ctrl.Result{}, err
        }

        if clusterSudoRequest.ObjectMeta.Labels == nil {
            clusterSudoRequest.ObjectMeta.Labels = make(map[string]string)
        }
        clusterSudoRequest.ObjectMeta.Labels["tarbac.io/request-id"] = requestId
        // Update the object with the new label
        if err := r.Client.Update(ctx, &clusterSudoRequest); err != nil {
            logger.Error(err, "Failed to update SudoRequest with RequestID label", "ClusterSudoRequest", clusterSudoRequest.Name)
            return ctrl.Result{}, err
        }
    }
    if clusterSudoRequest.Status.State == "Rejected" || clusterSudoRequest.Status.State == "Expired" {
		logger.Info("ClusterSudoRequest already processed", "state", clusterSudoRequest.Status.State)
		return ctrl.Result{}, nil
	}
	if clusterSudoRequest.Status.State == "Pending" {

        maxDuration, err := time.ParseDuration(clusterSudoPolicy.Spec.MaxDuration)
        if err != nil || duration > maxDuration {
            return r.rejectRequest(ctx, &clusterSudoRequest, fmt.Sprintf("Requested duration %s exceeds max allowed duration %s", duration, maxDuration), logger)
        }

        if !r.validateRequester(clusterSudoPolicy, requester) {
            return r.rejectRequest(ctx, &clusterSudoRequest, "User not allowed by policy", logger)
        }

        namespaces, err := r.getAllowedNamespaces(ctx, &clusterSudoPolicy)
        if err != nil {
            logger.Error(err, "Failed to retrieve allowed namespaces")
            return ctrl.Result{}, err
        }

        if clusterSudoRequest.Status.ChildResource == nil {
            clusterSudoRequest.Status.ChildResource = []v1.ChildResource{}
        }

        if len(namespaces) == 0 {
            return r.rejectRequest(ctx, &clusterSudoRequest, "No namespaces matched policy constraints", logger)
        }
        if len(namespaces) == 1 && namespaces[0] == "*" {
            r.Recorder.Event(&clusterSudoRequest, "Normal", "Approved", fmt.Sprintf("ClusterTemporaryRBAC created for User: %s using %s ClusterSudoPolicy [UID: %s]", requester, clusterSudoPolicy.Name, clusterSudoRequest.Status.RequestID))
            return r.createClusterTemporaryRBAC(ctx, &clusterSudoRequest, &clusterSudoPolicy, duration, logger)
        }
        if len(namespaces) >= 1 {
            r.Recorder.Event(&clusterSudoRequest, "Normal", "Approved", fmt.Sprintf("TemporaryRBAC created for User: %s using %s ClusterSudoPolicy [UID: %s]", requester, clusterSudoPolicy.Name, clusterSudoRequest.Status.RequestID))
            return r.createTemporaryRBACsForNamespaces(ctx, &clusterSudoRequest, namespaces, &clusterSudoPolicy, requester, duration, logger)
        }
	}

    if clusterSudoRequest.Status.State == "Approved" {
        logger.Info("ClusterSudoRequest is already approved, validating child resources")

        for _, childResource := range clusterSudoRequest.Status.ChildResource {

            if childResource.Name == "" {
                logger.Error(nil, "Child resource has incomplete data", "childResource", childResource)
                continue
            }

            // Fetch child resource
            switch childResource.Kind {
            case "TemporaryRBAC":
                var temporaryRBAC v1.TemporaryRBAC
                err := r.Get(ctx, client.ObjectKey{Name: childResource.Name, Namespace: childResource.Namespace}, &temporaryRBAC)
                if err != nil {
                    if apierrors.IsNotFound(err) {
                        logger.Error(err, "Child TemporaryRBAC resource not found", "child", childResource)
                        r.Recorder.Event(&clusterSudoRequest, "Warning", "MissingChildResource", fmt.Sprintf("Child resource %s/%s not found", childResource.Namespace, childResource.Name))
                        continue
                    }
                    logger.Error(err, "Failed to fetch child resource", "child", childResource)
                    return ctrl.Result{}, err
                }
                if temporaryRBAC.Status.CreatedAt != nil && clusterSudoRequest.Status.CreatedAt == nil {
                    clusterSudoRequest.Status.CreatedAt = temporaryRBAC.Status.CreatedAt
                }
                if temporaryRBAC.Status.ExpiresAt != nil && clusterSudoRequest.Status.ExpiresAt == nil {
                    clusterSudoRequest.Status.ExpiresAt = temporaryRBAC.Status.ExpiresAt
                }
                // Check the state of the child resource
                switch temporaryRBAC.Status.State {
                case "Expired":
                    clusterSudoRequest.Status.State = "Expired"
                    if err := r.Status().Update(ctx, &clusterSudoRequest); err != nil {
                        logger.Error(err, "Failed to update expired ClusterSudoRequest status")
                        return ctrl.Result{}, err
                    }
                    r.Recorder.Event(&clusterSudoRequest, "Warning", "Expired", fmt.Sprintf("ClusterSudoRequest Expired for User %s, revoked permissions for policy %s [UID: %s]", clusterSudoRequest.Annotations["tarbac.io/requester"], clusterSudoRequest.Spec.Policy, clusterSudoRequest.Status.RequestID))
                    logger.Info("ClusterSudoRequest has expired", "name", clusterSudoRequest.Name)
                    return ctrl.Result{}, nil
                case "Error":
                    clusterSudoRequest.Status.State = "Error"
                    if err := r.Status().Update(ctx, &clusterSudoRequest); err != nil {
                        logger.Error(err, "Failed to update error ClusterSudoRequest status")
                        return ctrl.Result{}, err
                    }
                    r.Recorder.Event(&clusterSudoRequest, "Error", "Error", fmt.Sprintf("Error detected while processing ClusterSudoRequest for User '%s' and policy '%s' [UID: %s]", clusterSudoRequest.Annotations["tarbac.io/requester"], clusterSudoRequest.Spec.Policy, clusterSudoRequest.Status.RequestID))
                    logger.Info("ClusterSudoRequest has expired", "name", clusterSudoRequest.Name)
                    return ctrl.Result{}, nil
                }
            case "ClusterTemporaryRBAC":
                var clusterTemporaryRBAC v1.ClusterTemporaryRBAC
                err := r.Get(ctx, client.ObjectKey{Name: childResource.Name}, &clusterTemporaryRBAC) // Cluster-scoped, no namespace
                if err != nil {
                    if apierrors.IsNotFound(err) {
                        logger.Error(err, "Child ClusterTemporaryRBAC resource not found", "child", childResource)
                        r.Recorder.Event(&clusterSudoRequest, "Warning", "MissingChildResource",
                            fmt.Sprintf("Child ClusterTemporaryRBAC resource %s not found", childResource.Name))
                        continue
                    }
                    logger.Error(err, "Failed to fetch child ClusterTemporaryRBAC resource", "child", childResource)
                    return ctrl.Result{}, err
                }
                if clusterTemporaryRBAC.Status.CreatedAt != nil && clusterSudoRequest.Status.CreatedAt == nil {
                    clusterSudoRequest.Status.CreatedAt = clusterTemporaryRBAC.Status.CreatedAt
                }
                if clusterTemporaryRBAC.Status.ExpiresAt != nil && clusterSudoRequest.Status.ExpiresAt == nil {
                    clusterSudoRequest.Status.ExpiresAt = clusterTemporaryRBAC.Status.ExpiresAt
                }

                // Check the state of the child resource
                switch clusterTemporaryRBAC.Status.State {
                case "Expired":
                    clusterSudoRequest.Status.State = "Expired"
                    if err := r.Status().Update(ctx, &clusterSudoRequest); err != nil {
                        logger.Error(err, "Failed to update expired ClusterSudoRequest status")
                        return ctrl.Result{}, err
                    }
                    r.Recorder.Event(&clusterSudoRequest, "Warning", "Expired", fmt.Sprintf("ClusterSudoRequest Expired for User %s, revoked permissions for policy %s [UID: %s]", clusterSudoRequest.Annotations["tarbac.io/requester"], clusterSudoRequest.Spec.Policy, clusterSudoRequest.Status.RequestID))
                    logger.Info("ClusterSudoRequest has expired", "name", clusterSudoRequest.Name)
                    return ctrl.Result{}, nil
                case "Error":
                    clusterSudoRequest.Status.State = "Error"
                    if err := r.Status().Update(ctx, &clusterSudoRequest); err != nil {
                        logger.Error(err, "Failed to update error ClusterSudoRequest status")
                        return ctrl.Result{}, err
                    }
                    r.Recorder.Event(&clusterSudoRequest, "Error", "Error", fmt.Sprintf("Error detected while processing ClusterSudoRequest for User '%s' and policy '%s' [UID: %s]", clusterSudoRequest.Annotations["tarbac.io/requester"], clusterSudoRequest.Spec.Policy, clusterSudoRequest.Status.RequestID))
                    logger.Info("ClusterSudoRequest has expired", "name", clusterSudoRequest.Name)
                    return ctrl.Result{}, nil
                }
                }
            }

        // Update the ClusterSudoRequest status
        if err := r.Client.Status().Update(ctx, &clusterSudoRequest); err != nil {
            logger.Error(err, "Failed to update ClusterSudoRequest status after child resource validation")
            return ctrl.Result{}, err
        }
        logger.Info("ClusterSudoRequest status updated based on child resources", "state", clusterSudoRequest.Status.State)
    }

    if clusterSudoRequest.Status.ExpiresAt != nil {
        timeUntilExpiration := time.Until(clusterSudoRequest.Status.ExpiresAt.Time)
        if timeUntilExpiration > 0 {
            logger.Info("Requeueing for expiration check", "timeUntilExpiration", timeUntilExpiration)
            return ctrl.Result{RequeueAfter: timeUntilExpiration}, nil
        }
        logger.Info("TimeUntilExpiration is negative or zero; setting state to Expired immediately", "timeUntilExpiration", timeUntilExpiration)
        clusterSudoRequest.Status.State = "Expired"
        if err := r.Client.Status().Update(ctx, &clusterSudoRequest); err != nil {
            logger.Error(err, "Failed to update ClusterSudoRequest status to Expired")
            return ctrl.Result{}, err
        }
        r.Recorder.Event(&clusterSudoRequest, "Warning", "Expired", fmt.Sprintf("ClusterSudoRequest of user '%s' for policy '%s' expired [UID: %s]", requester, clusterSudoPolicy.Name, clusterSudoRequest.Status.RequestID))
    }
    logger.Info("No expiration time set; skipping requeue")
    return ctrl.Result{}, nil
}

func (r *ClusterSudoRequestReconciler) getAllowedNamespaces(ctx context.Context, clusterSudoPolicy *v1.ClusterSudoPolicy) ([]string, error) {
	if clusterSudoPolicy.Spec.AllowedNamespacesSelector != nil {
		var namespaces corev1.NamespaceList
		if err := r.List(ctx, &namespaces, client.MatchingLabels(clusterSudoPolicy.Spec.AllowedNamespacesSelector.MatchLabels)); err != nil {
			return nil, err
		}
		var namespaceNames []string
		for _, ns := range namespaces.Items {
			namespaceNames = append(namespaceNames, ns.Name)
		}
		return namespaceNames, nil
	}

	return clusterSudoPolicy.Spec.AllowedNamespaces, nil
}

func (r *ClusterSudoRequestReconciler) createTemporaryRBACsForNamespaces(ctx context.Context, clusterSudoRequest *v1.ClusterSudoRequest, namespaces []string, clusterSudoPolicy *v1.ClusterSudoPolicy, requester string, duration time.Duration, logger logr.Logger) (ctrl.Result, error) {
    var childResources []v1.ChildResource

    for _, namespace := range namespaces {
        temporaryRBAC := &v1.TemporaryRBAC{
            ObjectMeta: metav1.ObjectMeta{
                Name:      fmt.Sprintf("temporaryrbac-%s-%s", clusterSudoRequest.Name, namespace),
                Namespace: namespace,
            },
            Spec: v1.TemporaryRBACSpec{
                Subjects: []rbacv1.Subject{
                    {
                        Kind: "User",
                        Name: requester,
                    },
                },
                RoleRef:  clusterSudoPolicy.Spec.RoleRef,
                Duration: clusterSudoRequest.Spec.Duration,
            },
        }

        if err := controllerutil.SetControllerReference(clusterSudoRequest, temporaryRBAC, r.Scheme); err != nil {
            logger.Error(err, "Failed to set OwnerReference on TemporaryRBAC", "namespace", namespace)
            continue
        }

        if err := r.Client.Create(ctx, temporaryRBAC); err != nil {
            logger.Error(err, "Failed to create TemporaryRBAC", "namespace", namespace)
            continue
        }

        logger.Info("TemporaryRBAC created successfully", "TemporaryRBAC", temporaryRBAC.Name, "namespace", namespace)

        childResources = append(childResources, v1.ChildResource{
            APIVersion:   "tarbac.io/v1",
            Kind:       "TemporaryRBAC",
            Name:       temporaryRBAC.Name,
            Namespace:  namespace,
        })
    }

    clusterSudoRequest.Status.State = "Approved"
    clusterSudoRequest.Status.ChildResource = childResources

    if err := r.Status().Update(ctx, clusterSudoRequest); err != nil {
        logger.Error(err, "Failed to update ClusterSudoRequest status with TemporaryRBAC details")
        return ctrl.Result{}, err
    }
    logger.Info("Successfully updated ClusterSudoRequest status with TemporaryRBAC details, requeuing while waiting for child resource creation")
    return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func (r *ClusterSudoRequestReconciler) createClusterTemporaryRBAC(ctx context.Context, clusterSudoRequest *v1.ClusterSudoRequest, clusterSudoPolicy *v1.ClusterSudoPolicy, duration time.Duration, logger logr.Logger) (ctrl.Result, error) {
	var childResources []v1.ChildResource

	clusterTemporaryRBAC := &v1.ClusterTemporaryRBAC{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("cluster-temporaryrbac-%s", clusterSudoRequest.Name),
		},
		Spec: v1.TemporaryRBACSpec{
			Subjects: []rbacv1.Subject{
				{
					Kind: "User",
					Name: clusterSudoRequest.Annotations["tarbac.io/requester"],
				},
			},
			RoleRef:  clusterSudoPolicy.Spec.RoleRef,
			Duration: clusterSudoRequest.Spec.Duration,
		},
	}

	if err := controllerutil.SetControllerReference(clusterSudoRequest, clusterTemporaryRBAC, r.Scheme); err != nil {
		logger.Error(err, "Failed to set OwnerReference on ClusterTemporaryRBAC")
		return ctrl.Result{}, err
	}

	if err := r.Create(ctx, clusterTemporaryRBAC); err != nil {
		logger.Error(err, "Failed to create ClusterTemporaryRBAC")
		return ctrl.Result{}, err
	}

	clusterSudoRequest.Status.State = "Approved"
    childResources = append(childResources, v1.ChildResource{
        APIVersion: "tarbac.io/v1",
        Kind:       "ClusterTemporaryRBAC",
        Name:       clusterTemporaryRBAC.Name,
    })
    clusterSudoRequest.Status.ChildResource = childResources

	if err := r.Status().Update(ctx, clusterSudoRequest); err != nil {
		logger.Error(err, "Failed to update ClusterSudoRequest status with ClusterTemporaryRBAC details")
		return ctrl.Result{}, err
	}
    logger.Info("Successfully updated ClusterSudoRequest status with TemporaryRBAC details, requeuing while waiting for child resource creation")
    return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func (r *ClusterSudoRequestReconciler) validateRequester(policy v1.ClusterSudoPolicy, requester string) bool {
	for _, user := range policy.Spec.AllowedUsers {
		if user.Name == requester {
			return true
		}
	}
	return false
}

func (r *ClusterSudoRequestReconciler) rejectRequest(ctx context.Context, clusterSudoRequest *v1.ClusterSudoRequest, message string, logger logr.Logger) (ctrl.Result, error) {
	logger.Info("Rejecting ClusterSudoRequest", "reason", message)
	clusterSudoRequest.Status.State = "Rejected"
	clusterSudoRequest.Status.ErrorMessage = message
	if err := r.Status().Update(ctx, clusterSudoRequest); err != nil {
		logger.Error(err, "Failed to update ClusterSudoRequest status to Rejected")
		return ctrl.Result{}, err
	}
	r.Recorder.Event(clusterSudoRequest, "Warning", "Rejected", message)
	return ctrl.Result{}, nil
}

func (r *ClusterSudoRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Scheme = mgr.GetScheme()
	r.Recorder = mgr.GetEventRecorderFor("ClusterSudoRequestController")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ClusterSudoRequest{}).
		Complete(r)
}
