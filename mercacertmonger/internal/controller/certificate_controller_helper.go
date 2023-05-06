package controller

import (
	"context"
	"fmt"
	tlsv1 "igarridot/learning-k8s-controllers/mercacertmonger/api/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Secret builder
func ConstructSecretForCertificate(certificate *tlsv1.Certificate, r *CertificateReconciler) (*corev1.Secret, error) {
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
		fmt.Println("Unable so set controller reference on child object", err)
		return nil, err
	}

	return secret, nil
}

// Renewal business logic
func CertificateNeedsToBeRenewed(now, expireTime int64, certificate tlsv1.Certificate) bool {
	if expireTime-now < CertIsNearToExpire {
		fmt.Println("Certificate", certificate.Name, "needs to be renewed")
		return true
	}
	fmt.Println("Certificate", certificate.Name, "does not need to be renewed")
	return false
}

// Fake timestamp manager
func CertificateTimestampManager(now int64, certificate *tlsv1.Certificate) *tlsv1.Certificate {
	certificate.Status.ValidFrom = now
	certificate.Status.ValidTo = now + CertRenewalPeriod
	return certificate
}

// Update Certificate Status
func UpdateCertificateStatus(r *CertificateReconciler, ctx context.Context, certificate *tlsv1.Certificate) (ctrl.Result, error) {
	err := r.Status().Update(ctx, certificate)
	if err != nil {
		fmt.Println(err, "Unable to update Certificate", certificate.Name, "status")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// Create new Certificate secret
func CreateNewCertificateSecret(r *CertificateReconciler, ctx context.Context, secret *corev1.Secret, certificate *tlsv1.Certificate) (ctrl.Result, error) {
	err := r.Create(ctx, secret)
	if err != nil {
		fmt.Println(err, "Unable to create secret for Certificate", certificate.Name, "secret", secret)
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// Update already existing Certificate
func RenewCertificate(r *CertificateReconciler, ctx context.Context, secret *corev1.Secret, certificate *tlsv1.Certificate) (ctrl.Result, error) {
	err := r.Update(ctx, secret)
	if err != nil {
		fmt.Println(err, "Unable to create secret for Certificate", certificate.Name, "secret", secret)
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}
