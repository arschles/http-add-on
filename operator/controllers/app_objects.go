package controllers

import (
	"strconv"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	typedAppsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	typedCorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// The image registry to be used to download KEDA assets, it'll
	// be concatenated with the image names
	imageRegistry string = "khaosdoctor/"
	// Image name for the interceptor container
	interceptorImageName string = "keda-http-interceptor"
	// Image name for the external scaler container
	externalScalerImageName string = "keda-http-external-scaler"
)

type KedaHTTPExternalScaler struct {
	name string

}

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

	// set initial statuses
	httpso.Status = v1alpha1.HTTPScaledObjectStatus{
		ServiceStatus:      v1alpha1.Pending,
		DeploymentStatus:   v1alpha1.Pending,
		ScaledObjectStatus: v1alpha1.Pending,
		InterceptorStatus: v1alpha1.Pending,
		ExternalScalerStatus: v1alpha1.Pending,
		Ready:              false,
	}

	// Init K8s clients
	appsCl := rec.K8sCl.AppsV1().Deployments(userAppNamespace)
	coreCl := rec.K8sCl.CoreV1().Services(userAppNamespace)

	// CREATING THE USER APPLICATION
	appError := createUserApp(userAppName, userAppNamespace, userAppImage, userAppPort, appsCl, coreCl, logger, httpso)
	if appError != nil {
		return appError
	}

	// CREATING INTERNAL ADD-ON OBJECTS
	// Creating the dedicated interceptor
	_, err := createInterceptor(userAppName, userAppPort, userAppNamespace, appsCl, coreCl, logger, httpso)
	if err != nil {
		return err
	}

	// create dedicated external scaler for this app
	externalScalerName, err := createExternalScaler(userAppName, userAppNamespace, appsCl, coreCl, logger, httpso)
	if err != nil {
		return err
	}

	// create the KEDA core ScaledObject (not the HTTP one).
	// this needs to be submitted so that KEDA will scale the app's deployment
	createScaledObject(userAppName, userAppNamespace, externalScalerName, rec.K8sDynamicCl, logger, httpso)

	return nil
}

func createScaledObject (userAppName string,
	userAppNamespace string,
	externalScaledAddress string,
	K8sDynamicCl dynamic.Interface,
	logger logr.Logger,
	httpso *v1alpha1.HTTPScaledObject) error {
	coreScaledObject := k8s.NewScaledObject(
		userAppNamespace,
		userAppName,
		userAppName,
		externalScaledAddress,
	)
	// TODO: use r.Client here, not the dynamic one
	scaledObjectCl := k8s.NewScaledObjectClient(K8sDynamicCl)
	if _, err := scaledObjectCl.
		Namespace(userAppNamespace).
		Create(coreScaledObject, metav1.CreateOptions{}); err != nil {
		logger.Error(err, "Creating ScaledObject")
		httpso.Status.ScaledObjectStatus = v1alpha1.Error
		return err
	}
	httpso.Status.ScaledObjectStatus = v1alpha1.Created
	return nil
}

func createUserApp (userAppName string,
	userAppNamespace string,
	userAppImage string,
	userAppPort int32,
	appsCl typedAppsv1.DeploymentInterface,
	coreCl typedCorev1.ServiceInterface,
	logger logr.Logger,
	httpso *v1alpha1.HTTPScaledObject) error {
	deployment := k8s.NewDeployment(userAppNamespace, userAppName, userAppImage, userAppPort, []corev1.EnvVar{})
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
	return nil
}

func createInterceptor (userAppName string, userAppPort int32,
		userAppNamespace string,
		appsCl typedAppsv1.DeploymentInterface,
		coreCl typedCorev1.ServiceInterface,
		logger logr.Logger,
		httpso *v1alpha1.HTTPScaledObject) (string, error) {
	interceptorExtendedAppName := userAppName + "-interceptor"
	interceptorEnvs := []corev1.EnvVar{
		{
			Name: "KEDA_HTTP_SERVICE_NAME",
			Value: userAppName,
		},
		{
			Name: "KEDA_HTTP_SERVICE_PORT",
			Value: strconv.FormatInt(int64(userAppPort), 10),
		},
	}

	// NOTE: Interceptor port is fixed here because it's a fixed on the interceptor main (@see ../interceptor/main.go:49)
	interceptorDeployment := k8s.NewDeployment(userAppNamespace, interceptorExtendedAppName, imageRegistry + interceptorImageName, 8080, interceptorEnvs)
	if _, err := appsCl.Create(interceptorDeployment); err != nil {
		logger.Error(err, "Creating interceptor deployment")
		httpso.Status.InterceptorStatus = v1alpha1.Error
		return "", err
	}

	interceptorService := k8s.NewService(userAppNamespace, interceptorExtendedAppName, 8080)
	if _, err := coreCl.Create(interceptorService); err != nil {
		logger.Error(err, "Creating interceptor service")
		httpso.Status.InterceptorStatus = v1alpha1.Error
		return "", err
	}
	httpso.Status.InterceptorStatus = v1alpha1.Created
	return interceptorExtendedAppName, nil
}

func createExternalScaler (userAppName string, userAppNamespace string,
		appsCl typedAppsv1.DeploymentInterface,
		coreCl typedCorev1.ServiceInterface,
		logger logr.Logger,
		httpso *v1alpha1.HTTPScaledObject) (string, error) {
	scalerExtendedAppName := userAppName + "-ext-scaler"
	// NOTE: Scaler port is fixed here because it's a fixed on the scaler main (@see ../scaler/main.go:17)
	scalerDeployment := k8s.NewDeployment(userAppNamespace, scalerExtendedAppName, imageRegistry + externalScalerImageName, 8080, []corev1.EnvVar{})
	if _, err := appsCl.Create(scalerDeployment); err != nil {
		logger.Error(err, "Creating scaler deployment")
		httpso.Status.ExternalScalerStatus = v1alpha1.Error
		return "", err
	}

	scalerService := k8s.NewService(userAppNamespace, scalerExtendedAppName, 8080)
	if _, err := coreCl.Create(scalerService); err != nil {
		logger.Error(err, "Creating scaler service")
		httpso.Status.ExternalScalerStatus = v1alpha1.Error
		return "", err
	}
	httpso.Status.ExternalScalerStatus = v1alpha1.Created
	return scalerExtendedAppName, nil
}
