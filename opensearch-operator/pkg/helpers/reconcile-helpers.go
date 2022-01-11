package helpers

import (
	"fmt"
	sts "k8s.io/api/apps/v1"
	opsterv1 "os-operator.io/api/v1"
)

func CheckUpdates(sts_env sts.StatefulSetSpec, sts_crd sts.StatefulSetSpec, instance *opsterv1.Os, count int) (x sts.StatefulSetSpec, scaled bool, err error) {

	fields := getNamesInStruct(sts_env)
	scaled = false

	for i := 0; i < len(fields); i++ {

		field := fields[i]
		field_env := getField(&sts_env, field)
		field_env_int_ptr, ok := field_env.(*int32)
		scaled = true
		if !ok {
			fmt.Println(!ok)
			return sts_env, scaled, err
		}
		if field_env_int_ptr == nil {
			return sts_env, false, err
		}
		field_env_int := *field_env_int_ptr

		field_crd := getField(&sts_crd, field)
		field_crd_int_ptr, ok := field_crd.(*int32)
		scaled = true
		if !ok {
			fmt.Println(!ok)
			return sts_env, false, err
		}
		if field_crd_int_ptr == nil {
			return sts_env, false, err
		}
		field_crd_int := *field_crd_int_ptr

		// Check if sts replica count from cluster is equal to what configured in CRD
		if field_env_int != field_crd_int {
			//if not equal - change env replica count to what configured in CRD
			scaled := true
			fmt.Println("You scaled - Replicas on " + instance.Spec.General.ClusterName + "-" + instance.Spec.OsNodes[count].Compenent)
			return sts_crd, scaled, nil
		}
	}
	return sts_env, false, nil

}

func CreateInitMasters(cr *opsterv1.Os) string {
	NodesCount := len(cr.Spec.OsNodes)

	var i int32
	for x := 0; x > NodesCount; x++ {
		comp := cr.Spec.OsNodes[x].Compenent
		if comp == "masters" {
			i = cr.Spec.OsNodes[x].Replicas
		}
	}

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
