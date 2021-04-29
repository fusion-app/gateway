package utils

import (
	"fmt"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

var log = ctrl.Log.WithName("utils")

func MatchesLabelSelector(slice map[string]string, selector map[string]string) bool {
	if len(slice) == 0 {
		return false
	}
	for key, expected := range selector {
		if raw, exists := slice[key]; !exists || raw != expected {
			return false
		}
	}
	return true
}

func ToUnstructured(runtimeObj interface{}) *unstructured.Unstructured {
	innerObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(runtimeObj)
	if err != nil {
		log.Error(err, "Convert to unstructured failed")
		return nil
	}
	return &unstructured.Unstructured{Object: innerObj}
}

func JSONSchemaID(u *unstructured.Unstructured) string {
	return fmt.Sprintf("%s/%s", u.GetAPIVersion(), u.GetKind())
}