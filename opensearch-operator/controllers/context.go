package controllers

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// Used to share state between different subcontrollers
type ControllerContext struct {
	Volumes          []corev1.Volume
	VolumeMounts     []corev1.VolumeMount
	OpenSearchConfig map[string]string
}

func NewControllerContext() ControllerContext {
	return ControllerContext{OpenSearchConfig: make(map[string]string)}
}

func (c *ControllerContext) AddConfig(key string, value string) {
	_, exists := c.OpenSearchConfig[key]
	if exists {
		fmt.Printf("Warning: Config key '%s' already exists. Will be overwritten", key)
	}
	c.OpenSearchConfig[key] = value
}
