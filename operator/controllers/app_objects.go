package controllers

import (
	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (rec *HTTPScaledObjectReconciler) removeAppObjects(
	logger logr.Logger,
	req ctrl.Request,
	httpso *v1alpha1.HTTPScaledObject,
) error {
	appName := httpso.Spec.AppName

	// set initial statuses
	httpso.Status = v1alpha1.HTTPScaledObjectStatus{
		ServiceStatus:      v1alpha1.Terminating,
		DeploymentStatus:   v1alpha1.Terminating,
		ScaledObjectStatus: v1alpha1.Terminating,
		InterceptorStatus: v1alpha1.Terminating,
		ExternalScalerStatus: v1alpha1.Terminating,
		Ready:              false,
	}
	logger = rec.Log.WithValues("reconciler.appObjects", "removeObjects", "HTTPScaledObject.name", appName, "HTTPScaledObject.namespace", httpso.Namespace)

	appsCl := rec.K8sCl.AppsV1().Deployments(req.Namespace)
	if err := appsCl.Delete(appName, &metav1.DeleteOptions{}); err != nil {
		logger.Error(err, "Deleting deployment")
		httpso.Status.DeploymentStatus = v1alpha1.Error
		return err
	}
	httpso.Status.DeploymentStatus = v1alpha1.Deleted

	coreCl := rec.K8sCl.CoreV1().Services(req.Namespace)
	if err := coreCl.Delete(appName, &metav1.DeleteOptions{}); err != nil {
		logger.Error(err, "Deleting service")
		httpso.Status.ServiceStatus = v1alpha1.Error
		return err
	}
	httpso.Status.ServiceStatus = v1alpha1.Deleted

	// TODO: use r.Client here, not the dynamic one
	scaledObjectCl := k8s.NewScaledObjectClient(rec.K8sDynamicCl)
	if err := scaledObjectCl.Namespace(req.Namespace).Delete(appName, &metav1.DeleteOptions{}); err != nil {
		logger.Error(err, "Deleting scaledobject")
		httpso.Status.ScaledObjectStatus = v1alpha1.Error
		return err
	}
	httpso.Status.ScaledObjectStatus = v1alpha1.Deleted
	return nil
}

func (rec *HTTPScaledObjectReconciler) addAppObjects(
	logger logr.Logger,
	req ctrl.Request,
	httpso *v1alpha1.HTTPScaledObject,
) error {
	// Gather initial data
	userAppName := httpso.Spec.AppName
	userAppImage := httpso.Spec.Image
	userAppPort := httpso.Spec.Port
	userAppNamespace := httpso.Namespace
	logger = rec.Log.WithValues("reconciler.appObjects", "addObjects", "HTTPScaledObject.name", userAppName, "HTTPScaledObject.namespace", userAppNamespace)
	httpso.Status = v1alpha1.HTTPScaledObjectStatus{
		ServiceStatus:      v1alpha1.Pending,
		DeploymentStatus:   v1alpha1.Pending,
		ScaledObjectStatus: v1alpha1.Pending,
		Ready:              false,
	}

	// CREATING THE USER APPLICATION
	deployment := k8s.NewDeployment(userAppNamespace, userAppName, userAppImage, userAppPort, []v1.EnvVar{})
	// TODO: watch the deployment until it reaches ready state
	// Option: start the creation here and add another method to check if the resources are created
	if _, err := appsCl.Create(deployment); err != nil {
		logger.Error(err, "Creating deployment")
		httpso.Status.DeploymentStatus = v1alpha1.Error
		return err
	}
	httpso.Status.DeploymentStatus = v1alpha1.Created

	service := k8s.NewService(userAppNamespace, userAppName, userAppPort)
	if _, err := coreCl.Create(service); err != nil {
		logger.Error(err, "Creating service")
		httpso.Status.ServiceStatus = v1alpha1.Error
		return err
	}
	httpso.Status.ServiceStatus = v1alpha1.Created

	// create the KEDA core ScaledObject (not the HTTP one).
	// this needs to be submitted so that KEDA will scale the app's
	// deployment
	coreScaledObject := k8s.NewScaledObject(
		userAppNamespace,
		req.Name,
		req.Name,
		rec.ExternalScalerAddress,
	)

	// TODO: use r.Client here, not the dynamic one
	scaledObjectCl := k8s.NewScaledObjectClient(rec.K8sDynamicCl)
	if _, err := scaledObjectCl.
		Namespace(userAppNamespace).
		Create(coreScaledObject, metav1.CreateOptions{}); err != nil {
		logger.Error(err, "Creating scaledobject")
		httpso.Status.ScaledObjectStatus = v1alpha1.Error
		return err
	}
	httpso.Status.ScaledObjectStatus = v1alpha1.Created

	return nil

	// TODO: install a dedicated interceptor deployment for this app
	// TODO: install a dedicated external scaler for this app
}
