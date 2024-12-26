/*
Copyright 2024.

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

package controller

import (
	"context"

	errors "k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	epamcomv1alpha1 "github.com/mkosterin/web-operator/api/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WebReconciler reconciles a Web object
type WebReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=epam.com,resources=webs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=epam.com,resources=webs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=epam.com,resources=webs/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Web object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.4/pkg/reconcile
func (r *WebReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var web epamcomv1alpha1.Web
	if err := r.Get(ctx, req.NamespacedName, &web); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Web resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch Web")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	cm := &corev1.ConfigMap{}
	err_cm := r.Get(ctx, client.ObjectKey{Name: web.Name + "-cm", Namespace: web.Namespace}, cm)
	if err_cm != nil && errors.IsNotFound(err_cm) {
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      web.Name + "-cm",
				Namespace: web.Namespace,
			},
			Data: map[string]string{
				"index.html": web.Spec.HtmlContent,
			},
		}
		if err := ctrl.SetControllerReference(&web, cm, r.Scheme); err != nil {
			log.Error(err, "unable to set owner reference on ConfigMap")
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, cm); err != nil && !errors.IsAlreadyExists(err) {
			log.Error(err, "unable to create ConfigMap for Web", "configMap", cm)
			return ctrl.Result{}, err
		}
		log.Info("ConfigMap has been created", "configMap", cm.Name)

	} else if err_cm != nil {
		log.Error(err_cm, "unable to get ConfigMap")
		return ctrl.Result{}, err_cm
	}

	dep := &appsv1.Deployment{}
	err_dep := r.Get(ctx, client.ObjectKey{Name: web.Name + "deployment", Namespace: web.Namespace}, dep)
	if err_dep != nil && errors.IsNotFound(err_dep) {
		dep := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      web.Name + "deployment",
				Namespace: web.Namespace,
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": web.Name},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": web.Name,
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Image: web.Spec.Image,
								Name:  "web-container",
								VolumeMounts: []corev1.VolumeMount{
									{
										MountPath: "/app",
										Name:      "html",
									},
								},
							},
						},
					},
				},
			},
		}
		dep.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: "html",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: cm.Name,
						},
					},
				},
			},
		}
		if err := ctrl.SetControllerReference(&web, dep, r.Scheme); err != nil {
			log.Error(err, "unable to set owner reference on Deployment")
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, dep); err != nil && !errors.IsAlreadyExists(err) {
			log.Error(err, "unable to create Deployment for Web", "deployment", dep)
			return ctrl.Result{}, err
		}
		log.Info("Deployment has been created", "deployemnt", dep.Name)
	} else if err_dep != nil {
		log.Error(err_dep, "unable to get Deployment")
		return ctrl.Result{}, err_dep
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WebReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&epamcomv1alpha1.Web{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
