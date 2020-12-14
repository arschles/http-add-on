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
	so *v1alpha1.HTTPScaledObject,
) error {
	// TODO
	return nil
}

func (rec *HTTPScaledObjectReconciler) addAppObjects(
	logger logr.Logger,
	req ctrl.Request,
	httpso *v1alpha1.HTTPScaledObject,
) error {
	logger = rec.Log.WithValues()
	appName := httpso.Spec.AppName
	image := httpso.Spec.Image
	port := httpso.Spec.Port

	appsCl := rec.K8sCl.AppsV1().Deployments(req.Namespace)
	deployment := k8s.NewDeployment(req.Namespace, appName, image, port)
	// TODO: watch the deployment until it reaches ready state
	if _, err := appsCl.Create(deployment); err != nil {
		logger.Error(err, "Creating deployment")
		return err
	}

	coreCl := rec.K8sCl.CoreV1().Services(req.Namespace)
	service := k8s.NewService(req.Namespace, appName, port)
	if _, err := coreCl.Create(service); err != nil {
		logger.Error(err, "Creating service")
		return err
	}

	// create the KEDA core ScaledObject (not the HTTP one).
	// this needs to be submitted so that KEDA will scale the app's
	// deployment
	coreScaledObject := k8s.NewScaledObject(
		req.Namespace,
		req.Name,
		req.Name,
		rec.ExternalScalerAddress,
	)
	// TODO: use r.Client here, not the dynamic one
	scaledObjectCl := k8s.NewScaledObjectClient(rec.K8sDynamicCl)
	if _, err := scaledObjectCl.
		Namespace(req.Namespace).
		Create(coreScaledObject, metav1.CreateOptions{}); err != nil {
		logger.Error(err, "Creating scaledobject")
		return err
	}

	return nil

	// TODO: install a dedicated interceptor deployment for this app
	// TODO: install a dedicated external scaler for this app
}
