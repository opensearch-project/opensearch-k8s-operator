package helpers

import (
	"context"
	"fmt"
	opsterv1 "opensearch.opster.io/api/v1"
	"reflect"

	sts "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type OpenSearchReconciler struct {
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

func (r *OpenSearchReconciler) UpdateResource(ctx context.Context, instance *sts.StatefulSet) error {
	err := r.Update(ctx, instance)
	if err != nil {
		fmt.Println(err, "Cannot update resource")
		r.Recorder.Event(instance, "Warning", "Cannot update resource", "Cannot update resource")
		return err
	}
	return nil
}

func GetField(v *sts.StatefulSetSpec, field string) interface{} {

	r := reflect.ValueOf(v)
	f := reflect.Indirect(r).FieldByName(field).Interface()
	return f
}

//func setField(v *sts.StatefulSetSpec, field string) reflect.Value {
//
//	//	reflect.ValueOf(v).Elem().FieldByName(field).SetString("sss")
//
//	r := reflect.ValueOf(v)
//	ty := r.Type()
//	fmt.Println(ty)
//	f := reflect.Indirect(r).FieldByName(field)
//	return f
//}

func getNamesInStruct(inter interface{}) []string {
	rv := reflect.Indirect(reflect.ValueOf(inter))

	var names []string

	for i := 0; i < rv.NumField(); i++ {
		x := rv.Type().Field(i).Name
		names = append(names, x)
	}
	return names
}

func FindFirstPartial(arr []opsterv1.ComponentsStatus, item opsterv1.ComponentsStatus, predicator func(opsterv1.ComponentsStatus, opsterv1.ComponentsStatus) (opsterv1.ComponentsStatus, bool)) (opsterv1.ComponentsStatus, bool) {
	for i := 0; i < len(arr); i++ {
		itemInArr, found := predicator(arr[i], item)
		if found {
			return itemInArr, found
		}
	}
	return item, false
}

func Replace(remove opsterv1.ComponentsStatus, add opsterv1.ComponentsStatus, ssSlice []opsterv1.ComponentsStatus) []opsterv1.ComponentsStatus {
	removedSlice := RemoveIt(remove, ssSlice)
	fullSliced := append(removedSlice, add)
	return fullSliced
}

func RemoveIt(ss opsterv1.ComponentsStatus, ssSlice []opsterv1.ComponentsStatus) []opsterv1.ComponentsStatus {
	for idx, v := range ssSlice {
		if v == ss {
			return append(ssSlice[0:idx], ssSlice[idx+1:]...)
		}
	}
	return ssSlice
}

func Remove(slice []opsterv1.ComponentsStatus, componenet string) []opsterv1.ComponentsStatus {
	emptyStatus := opsterv1.ComponentsStatus{
		Component:   "",
		Status:      "",
		Description: "",
	}
	for i := 0; i < len(slice); i++ {
		if slice[i].Component == componenet {
			slice[i] = slice[len(slice)-1]    // Copy last element to index i.
			slice[len(slice)-1] = emptyStatus // Erase last element (write zero value).
			slice = slice[:len(slice)-1]
		}

	}
	return slice
}

// Find key in interface (recursively) and return value as interface
func Find(obj interface{}, key string) (interface{}, bool) {

	//if the argument is not a map, ignore it
	mobj, ok := obj.(map[string]interface{})
	if !ok {
		return nil, false
	}

	for k, v := range mobj {
		// key match, return value
		if k == key {
			return v, true
		}

		// if the value is a map, search recursively
		if m, ok := v.(map[string]interface{}); ok {
			if res, ok := Find(m, key); ok {
				return res, true
			}
		}
		// if the value is an array, search recursively
		// from each element
		if va, ok := v.([]interface{}); ok {
			for _, a := range va {
				if res, ok := Find(a, key); ok {
					return res, true
				}
			}
		}
	}

	// element not found
	return nil, false
}
func FindByPath(obj interface{}, keys []string) (interface{}, bool) {
	mobj, ok := obj.(map[string]interface{})
	if !ok {
		return nil, false
	}
	for i := 0; i < len(keys)-1; i++ {
		if currentVal, found := mobj[keys[i]]; found {
			subPath, ok := currentVal.(map[string]interface{})
			if !ok {
				return nil, false
			}
			mobj = subPath
		}
	}
	val, ok := mobj[keys[len(keys)-1]].(interface{})
	return val, ok
}

func CopyAndExclude(arr []string, itemToExclude string) []string {
	new_arr := make([]string, 0, len(arr))
	for i := 0; i < len(arr); i++ {
		if arr[i] != itemToExclude {
			new_arr = append(new_arr, arr[i])
		}
	}
	return new_arr
}
