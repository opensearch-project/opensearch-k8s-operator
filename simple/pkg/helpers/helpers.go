package helpers

import (
	"fmt"
	opsterv1alpha1 "opster.io/es/api/v1alpha1"
)

func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false

}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

func CreateInitmasters(cr *opsterv1alpha1.Es) string {
	i := cr.Spec.Masters.Replicas
	p := int(i)

	var masters = ""
	for x := 0; x < p; x++ {
		masters = fmt.Sprintf("%s-master-%d,%s", cr.Spec.General.ClusterName, x, masters)
	}
	if last := len(masters) - 1; last >= 0 && masters[last] == ',' {
		masters = masters[:last]
	}
	return masters

}
