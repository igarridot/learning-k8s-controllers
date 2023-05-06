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

const CertRenewalPeriod = 90
const CertIsNearToExpire = 30

//+kubebuilder:rbac:groups=tls.igarrido.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tls.igarrido.io,resources=certificates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tls.igarrido.io,resources=certificates/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=secrets/status,verbs=get;update;patch

// Here we go with the reconcile magic
func (r *CertificateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	fmt.Println("Starting reconciliation process")

	// Load the Certificate by name
	var certificate tlsv1.Certificate
	if err := r.Get(ctx, req.NamespacedName, &certificate); err != nil {
		fmt.Println(err, "unable to fetch Certificate")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	fmt.Println("Reconciliation process tarted for certificate ", certificate.Name)

	// List all Secrets managed by the Certificate
	var childSecrets corev1.SecretList
	if err := r.List(ctx, &childSecrets, client.InNamespace(req.Namespace), client.MatchingFields{jobOwnerKey: req.Name}); err != nil {
		fmt.Println(err, "unable to list child secrets")
		return ctrl.Result{}, err
	}

	now := time.Now().Unix()

	if certificate.Status.ValidTo == 0 {
		fmt.Println("Creating certificate", certificate.Name)

		CertificateTimestampManager(now, &certificate)
		fmt.Println("Certificate", certificate.Name, "will be valid until: ", now+CertRenewalPeriod)

		_, err := UpdateCertificateStatus(r, ctx, &certificate)
		if err != nil {
			fmt.Println("Cannot update Certificate Status", err)
		}

		secret, err := ConstructSecretForCertificate(&certificate, r)
		if err != nil {
			fmt.Println(err, "Unable to construct secret from template")
			return ctrl.Result{}, nil
		}

		_, err = CreateNewCertificateSecret(r, ctx, secret, &certificate)
		if err != nil {
			fmt.Println("Cannot create certificate", err)
		}

		fmt.Println("End of certificate", certificate.Name, "creation")
	}

	if CertificateNeedsToBeRenewed(now, certificate.Status.ValidTo, certificate) {
		fmt.Println("Renewing certificate", certificate.Name)

		CertificateTimestampManager(now, &certificate)
		fmt.Println("Certificate", certificate.Name, "will be valid until: ", now+90)

		_, err := UpdateCertificateStatus(r, ctx, &certificate)
		if err != nil {
			fmt.Println("Cannot update Certificate Status", err)
		}

		secret, err := ConstructSecretForCertificate(&certificate, r)
		if err != nil {
			fmt.Println(err, "Unable to construct secret from template")
			return ctrl.Result{}, nil
		}

		_, err = RenewCertificate(r, ctx, secret, &certificate)
		if err != nil {
			fmt.Println("Cannot renew certificate", err)
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
