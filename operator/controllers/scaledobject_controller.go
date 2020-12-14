/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/api/v1alpha1"
)

// HTTPScaledObjectReconciler reconciles a HTTPScaledObject object
type HTTPScaledObjectReconciler struct {
	K8sCl                 *kubernetes.Clientset
	K8sDynamicCl          dynamic.Interface
	ExternalScalerAddress string
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=http.keda.sh,resources=scaledobjects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=http.keda.sh,resources=scaledobjects/status,verbs=get;update;patch

// Reconcile reconciles a newly created, deleted, or otherwise changed
// HTTPScaledObject
func (r *HTTPScaledObjectReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("HTTPScaledObject.Namespace", req.Namespace, "HTTPScaledObject.Name", req.Name)

	ctx := context.Background()
	_ = r.Log.WithValues("scaledobject", req.NamespacedName)
	so := &httpv1alpha1.HTTPScaledObject{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Name:      req.Name,
		Namespace: req.Namespace,
	}, so); err != nil {
		if errors.IsNotFound(err) {
			// If the HTTPScaledObject wasn't found, it might have
			// been deleted between the reconcile and the get.
			// It'll automatically get garbage collected, so don't
			// schedule a requeue
			return ctrl.Result{}, nil
		}
		// if we didn't get a not found error, log it and schedule a requeue
		// with a backoff
		logger.Error(err, "Getting the HTTP Scaled obj")
		return ctrl.Result{
			RequeueAfter: 500 * time.Millisecond,
		}, err
	}

	if so.GetDeletionTimestamp() != nil {
		// if it was marked deleted, delete all the related objects
		// and don't schedule for another reconcile. Kubernetes
		// will finalize them
		removeErr := r.removeAppObjects(logger, req, so)
		if removeErr != nil {
			logger.Error(removeErr, "Removing application objects")
		}
		return ctrl.Result{}, removeErr
	}

	appName := so.Spec.AppName
	image := so.Spec.Image
	port := so.Spec.Port
	logger.Info("App Name: %s, image: %s, port: %d", appName, image, port)

	if err := r.addAppObjects(logger, req, so); err != nil {
		logger.Error(err, "Adding app objects")
		// TODO: delete app objects that have been created already
		return ctrl.Result{}, err
	}
	// TODO: set statuses

	return ctrl.Result{
		// TODO: should we requeue immediately?
		RequeueAfter: time.Millisecond * 200,
	}, nil
}

// SetupWithManager starts up reconciliation with the given manager
func (r *HTTPScaledObjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&httpv1alpha1.HTTPScaledObject{}).
		Complete(r)
}
