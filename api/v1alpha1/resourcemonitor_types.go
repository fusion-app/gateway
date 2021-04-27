/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ResourceMonitorSpec defines the desired state of ResourceMonitor
type ResourceMonitorSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of ResourceMonitor. Edit resourcemonitor_types.go to remove/update
	Selector       SelectorSpec      `json:"selector"`
	ChangeBuilder  ChangeBuilderSpec `json:"changeBuilder"`
	MsgBackendSpec `json:",inline"`
}

type SelectorSpec struct {
	GVK       metav1.GroupVersionKind `json:"gvk"`
	Namespace string                  `json:"namespace,omitempty"`
	Labels    map[string]string       `json:"labels,omitempty"`
	//Annotations metav1.LabelSelector    `json:"annotations,omitempty"`
}

type ChangeBuilderSpec struct {
	Type ChangeFilterType `json:"type"`
}

type ChangeFilterType string

const (
	JSONDiff ChangeFilterType = "JSONDiff"
)

type MsgBackendSpec struct {
	MQTTBackend *MQTTBackendSpec `json:"mqttBackend,omitempty"`
}

type MQTTBackendSpec struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Topic    string `json:"topic"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// ResourceMonitorStatus defines the observed state of ResourceMonitor
type ResourceMonitorStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Selected int `json:"selected,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ResourceMonitor is the Schema for the resourcemonitors API
type ResourceMonitor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceMonitorSpec   `json:"spec,omitempty"`
	Status ResourceMonitorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ResourceMonitorList contains a list of ResourceMonitor
type ResourceMonitorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceMonitor `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ResourceMonitor{}, &ResourceMonitorList{})
}
