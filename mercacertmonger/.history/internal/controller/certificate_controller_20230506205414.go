/*
Copyright 2023.

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
	"fmt"
	"time"

	tlsv1 "igarridot/learning-k8s-controllers/mercacertmonger/api/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type CertificateReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=tls.igarrido.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tls.igarrido.io,resources=certificates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tls.igarrido.io,resources=certificates/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=secrets/status,verbs=get;update;patch

// Here we go with the reconcile magic
func (r *CertificateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// Load the Certificate by name
	var certificate tlsv1.Certificate
	if err := r.Get(ctx, req.NamespacedName, &certificate); err != nil {
		fmt.Println(err, "unable to fetch Certificate")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// List all Secrets managed by the Certificate
	var childSecrets corev1.SecretList
	if err := r.List(ctx, &childSecrets, client.InNamespace(req.Namespace), client.MatchingFields{jobOwnerKey: req.Name}); err != nil {
		fmt.Println(err, "unable to list child secrets")
		return ctrl.Result{}, err
	}

	// Secret builder
	constructSecretForCertificate := func(certificate *tlsv1.Certificate) (*corev1.Secret, error) {

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				Name:        certificate.Name,
				Namespace:   certificate.Namespace,
			},
			StringData: make(map[string]string),
		}
		if err := ctrl.SetControllerReference(certificate, secret, r.Scheme); err != nil {
			return nil, err
		}

		return secret, nil
	}

	now := time.Now().Unix()

	var certificateNeedsToBeRenewed = func(now, expireTime int64) bool {
		if expireTime-now < 30 {
			certificateNeedsToBeRenewed := true
			fmt.Println("Certificate", certificate.Name, "needs to be renewed")
			return certificateNeedsToBeRenewed
		}
		certificateNeedsToBeRenewed := false
		fmt.Println("Certificate", certificate.Name, "does not need to be renewed")
		return certificateNeedsToBeRenewed
	}

	if certificate.Status.ValidTo == 0 {
		fmt.Println("Creating certificate", certificate.Name)

		certificate.Status.ValidFrom = now
		certificate.Status.ValidTo = now + 90
		fmt.Println("Certificate", certificate.Name, "will be valid until: ", now+50)

		// Update the CRD status
		if err := r.Status().Update(ctx, &certificate); err != nil {
			fmt.Println(err, "Unable to update Certificate", certificate.Name, "status")
			return ctrl.Result{}, err
		}

		// actually make the job...
		secret, err := constructSecretForCertificate(&certificate)
		if err != nil {
			fmt.Println(err, "Unable to construct secret from template")
			return ctrl.Result{}, nil
		}

		// ...and create it on the cluster
		if err := r.Create(ctx, secret); err != nil {
			fmt.Println(err, "Unable to create secret for Certificate", certificate.Name, "secret", secret)
			return ctrl.Result{}, err
		}

		fmt.Println("End of certificate", certificate.Name, "creation")
	}

	if certificateNeedsToBeRenewed(now, certificate.Status.ValidTo) {
		fmt.Println("Renewing certificate", certificate.Name)

		certificate.Status.ValidFrom = now
		certificate.Status.ValidTo = now + 50
		fmt.Println("Certificate", certificate.Name, "will be valid until: ", now+50)

		if err := r.Status().Update(ctx, &certificate); err != nil {
			fmt.Println(err, "unable to update Certificate", certificate.Name, "status")
			return ctrl.Result{}, err
		}

		secret, err := constructSecretForCertificate(&certificate)
		if err != nil {
			fmt.Println(err, "unable to construct secret from template")
			return ctrl.Result{}, nil
		}

		if err := r.Update(ctx, secret); err != nil {
			fmt.Println(err, "unable to create secret for Certificate", certificate.Name, "secret", secret)
			return ctrl.Result{}, err
		}

		fmt.Println("End of certificate", certificate.Name, "renewal")
	}

	return ctrl.Result{}, nil
}

var (
	jobOwnerKey = ".metadata.controller"
	apiGVStr    = corev1.SchemeGroupVersion.String()
)

// SetupWithManager sets up the controller with the Manager.
func (r *CertificateReconciler) SetupWithManager(mgr ctrl.Manager) error {

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.Secret{}, jobOwnerKey, func(rawObj client.Object) []string {
		// grab the job object, extract the owner...
		job := rawObj.(*corev1.Secret)
		owner := metav1.GetControllerOf(job)
		if owner == nil {
			return nil
		}
		// ...make sure it's a Certificate...
		if owner.APIVersion != apiGVStr || owner.Kind != "Certificate" {
			return nil
		}

		// ...and if so, return it
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&tlsv1.Certificate{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
