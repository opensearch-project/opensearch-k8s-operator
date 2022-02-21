package helpers

import "k8s.io/apimachinery/pkg/runtime"

// A simple mock to use whenever a record.EventRecorder is needed for a test
type MockEventRecorder struct {
}

func (r *MockEventRecorder) Event(object runtime.Object, eventtype, reason, message string) {

}

func (r *MockEventRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {

}

func (r *MockEventRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {

}
