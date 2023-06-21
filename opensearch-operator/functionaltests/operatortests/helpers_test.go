package operatortests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/util/intstr"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateKubernetesObjects(name string) error {
	data, err := os.ReadFile(name + ".yaml")
	if err != nil {
		return err
	}
	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(data), 100)
	for {
		var rawObj runtime.RawExtension
		if err = decoder.Decode(&rawObj); err != nil {
			break
		}

		obj, _, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
		if err != nil {
			log.Fatal(err)
		}
		unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			log.Fatal(err)
		}

		unstructuredObj := &unstructured.Unstructured{Object: unstructuredMap}

		if err = k8sClient.Create(context.Background(), unstructuredObj); err != nil {
			log.Fatal(err)
		}
	}
	if err != io.EOF {
		log.Fatal("eof ", err)
	}
	return nil
}

func Cleanup(name string) {
	data, err := os.ReadFile(name + ".yaml")
	if err != nil {
		log.Fatal(err)
	}
	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(data), 100)
	for {
		var rawObj runtime.RawExtension
		if err = decoder.Decode(&rawObj); err != nil {
			break
		}

		obj, _, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
		if err != nil {
			log.Fatal(err)
		}
		unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			log.Fatal(err)
		}

		unstructuredObj := &unstructured.Unstructured{Object: unstructuredMap}
		// Ignore errors as we don't care at this point
		_ = k8sClient.Delete(context.Background(), unstructuredObj)
	}
	if err != io.EOF {
		log.Fatal("eof ", err)
	}
}

func Get(obj client.Object, key client.ObjectKey, timeout time.Duration) {
	Eventually(func() bool {
		err := k8sClient.Get(context.Background(), key, obj)
		return err == nil
	}, timeout, time.Second*1).Should(BeTrue())
}

func ExposePodViaNodePort(selector map[string]string, namespace string, nodePort, targetPort int32) error {
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("nodeport-%d", nodePort),
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "nodeport",
					NodePort:   nodePort,
					Port:       targetPort,
					TargetPort: intstr.FromInt(int(targetPort)),
				},
			},
			Selector: selector,
			Type:     corev1.ServiceTypeNodePort,
		},
	}
	return k8sClient.Create(context.Background(), &service)
}

func CleanUpNodePort(namespace string, nodePort int32) error {
	service := corev1.Service{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: fmt.Sprintf("nodeport-%d", nodePort)}, &service); err != nil {
		return err
	}
	return k8sClient.Delete(context.Background(), &service)
}

func SetNestedKey(obj map[string]interface{}, value string, keys ...string) error {
	var m = obj
	for idx, key := range keys {
		if idx == len(keys)-1 {
			m[key] = value
			return nil
		} else {
			m = m[key].(map[string]interface{})
		}
	}
	return nil
}
