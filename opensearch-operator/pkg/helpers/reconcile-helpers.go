package helpers

import (
	"fmt"
	"strings"

	sts "k8s.io/api/apps/v1"
	"k8s.io/kube-openapi/pkg/validation/errors"
	opsterv1 "opensearch.opster.io/api/v1"
)

func CheckUpdates(sts_env sts.StatefulSetSpec, sts_crd sts.StatefulSetSpec, instance *opsterv1.OpenSearchCluster, count int, check string) (x sts.StatefulSetSpec, err error, changes []string) {

	fields := getNamesInStruct(sts_env)
	changes = []string{}

	//type fields_changes struct{
	//	nodegroup string `json:"nodegroup,omitempty"`
	//	change []string `json:"changes,omitempty"`
	//}
	//changes := []fields_changes{}

	for i := 0; i < len(fields); i++ {

		field := fields[i]
		field_env := GetField(&sts_env, field)
		field_env_int_ptr, ok := field_env.(*int32)
		if !ok {
			fmt.Println(!ok)
			return sts_env, err, changes
		}
		if field_env_int_ptr == nil {
			return sts_env, err, changes
		}
		field_env_int := *field_env_int_ptr

		field_crd := GetField(&sts_crd, field)
		field_crd_int_ptr, ok := field_crd.(*int32)
		if !ok {
			fmt.Println(!ok)
			return sts_env, err, changes
		}
		if field_crd_int_ptr == nil {
			return sts_env, err, changes
		}
		field_crd_int := *field_crd_int_ptr

		// Check if sts replica count from cluster is equal to what configured in CRD
		if field_env_int != field_crd_int {
			//if not equal - change env replica count to what configured in CRD
			changes = append(changes, field)

			//scaled := true
			//fmt.Println("You scaled - Replicas on " + instance.Spec.General.ClusterName + "-" + instance.Spec.nodePools[count].Component)
		}
	}
	return sts_env, nil, changes

}

func CreateInitMasters(cr *opsterv1.OpenSearchCluster) string {
	var masters []string
	for _, nodePool := range cr.Spec.NodePools {
		if ContainsString(nodePool.Roles, "master") {
			for i := 0; int32(i) < nodePool.Replicas; i++ {
				masters = append(masters, fmt.Sprintf("%s-%s-%d", cr.Spec.General.ClusterName, nodePool.Component, i))
			}
		}
	}
	return strings.Join(masters, ",")
}

func CheckEquels(from_env *sts.StatefulSetSpec, from_crd *sts.StatefulSetSpec, text string) (int32, bool, error) {
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
