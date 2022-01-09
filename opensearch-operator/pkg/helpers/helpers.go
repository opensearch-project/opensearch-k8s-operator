package helpers

import (
	"context"
	"fmt"
	sts "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	opsterv1 "os-operator.io/api/v1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type OsReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

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

func CreateInitmasters(cr *opsterv1.Os) string {
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

func (r *OsReconciler) UpdateResource(ctx context.Context, instance *sts.StatefulSet) error {
	err := r.Update(ctx, instance)
	if err != nil {
		fmt.Println(err, "Cannot update resource")
		r.Recorder.Event(instance, "Warning", "Cannot update resource", fmt.Sprintf("Cannot update resource "))
		return err
	}
	return nil
}

func getField(v *sts.StatefulSetSpec, field string) interface{} {

	r := reflect.ValueOf(v)
	f := reflect.Indirect(r).FieldByName(field).Interface()
	return f
}

func setField(v *sts.StatefulSetSpec, field string) reflect.Value {

	//	reflect.ValueOf(v).Elem().FieldByName(field).SetString("sss")

	r := reflect.ValueOf(v)
	ty := r.Type()
	fmt.Println(ty)
	f := reflect.Indirect(r).FieldByName(field)
	return f
}

func getNamesInStruct(inter interface{}) []string {
	rv := reflect.Indirect(reflect.ValueOf(inter))

	var names []string

	for i := 0; i < rv.NumField(); i++ {
		x := rv.Type().Field(i).Name
		names = append(names, x)
		fmt.Println(x)
	}
	return names
}

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
