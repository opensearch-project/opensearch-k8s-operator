package v1

import "k8s.io/apimachinery/pkg/types"

type OpensearchClusterSelector struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

func (o *OpensearchClusterSelector) ObjectKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      o.Name,
		Namespace: o.Namespace,
	}
}
