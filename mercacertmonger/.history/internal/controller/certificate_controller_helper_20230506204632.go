package controller

import (
	tlsv1 "igarridot/learning-k8s-controllers/mercacertmonger/api/v1"

	corev1 "k8s.io/api/core/v1"
)

func constructSecretForCertificate(certificate *tlsv1.Certificate) (*corev1.Secret, error) {

}
