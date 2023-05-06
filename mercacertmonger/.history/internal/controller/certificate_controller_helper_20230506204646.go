package controller

import (
	tlsv1 "igarridot/learning-k8s-controllers/mercacertmonger/api/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func constructSecretForCertificate(certificate *tlsv1.Certificate) (*corev1.Secret, error) {
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
