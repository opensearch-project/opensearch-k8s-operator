package controllers

import (
	corev1 "k8s.io/api/core/v1"
)

// Used to share state between different subcontrollers
type ControllerContext struct {
	Volumes          []corev1.Volume
	VolumeMounts     []corev1.VolumeMount
	OpenSearchConfig []string
}

func NewControllerContext() ControllerContext {
	return ControllerContext{}
}
