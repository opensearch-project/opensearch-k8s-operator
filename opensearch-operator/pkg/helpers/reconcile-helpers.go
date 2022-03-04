package helpers

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/kube-openapi/pkg/validation/errors"
	opsterv1 "opensearch.opster.io/api/v1"
)

func CreateInitMasters(cr *opsterv1.OpenSearchCluster) string {
	var masters []string
	for _, nodePool := range cr.Spec.NodePools {
		if ContainsString(nodePool.Roles, "master") {
			for i := 0; int32(i) < nodePool.Replicas; i++ {
				masters = append(masters, fmt.Sprintf("%s-%s-%d", cr.Name, nodePool.Component, i))
			}
		}
	}
	return strings.Join(masters, ",")
}

func CheckEquels(from_env *appsv1.StatefulSetSpec, from_crd *appsv1.StatefulSetSpec, text string) (int32, bool, error) {
	field_env := GetField(from_env, text)
	field_env_int_ptr, ok := field_env.(*int32)
	if !ok {
		err := errors.New(777, "something was worng")
		return *field_env_int_ptr, false, err
	}
	if field_env_int_ptr == nil {
		err := errors.New(777, "something was worng")
		return *field_env_int_ptr, false, err

	}
	field_crd := GetField(from_crd, "Replicas")
	field_crd_int_ptr, ok := field_crd.(*int32)
	if !ok {
		err := errors.New(777, "something was worng")
		return *field_crd_int_ptr, false, err
	}
	if field_crd_int_ptr == nil {
		err := errors.New(777, "something was worng")
		return *field_crd_int_ptr, false, err

	}

	if field_env_int_ptr != field_crd_int_ptr {
		return *field_crd_int_ptr, false, nil
	} else {
		return *field_crd_int_ptr, true, nil
	}
}
