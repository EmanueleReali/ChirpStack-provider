/*
Copyright 2022 The Crossplane Authors.

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
	"reflect"

	"google.golang.org/protobuf/proto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/chirpstack/chirpstack/api/go/v4/api"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// DeviceParameters are the configurable fields of a Device.
type DeviceParameters struct {
	DeviceStruct *api.Device `json:"device"`
}

// DeviceObservation are the observable fields of a Device.
type DeviceObservation struct {
	ObservableField string `json:"observableField,omitempty"`
}

// A DeviceSpec defines the desired state of a Device.
type DeviceSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       DeviceParameters `json:"forProvider"`
}

// A DeviceStatus represents the observed state of a Device.
type DeviceStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          DeviceObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Device is an example API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,chirpstack}
type Device struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeviceSpec   `json:"spec"`
	Status DeviceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DeviceList contains a list of Device
type DeviceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Device `json:"items"`
}

// Device type metadata.
var (
	DeviceKind             = reflect.TypeOf(Device{}).Name()
	DeviceGroupKind        = schema.GroupKind{Group: Group, Kind: DeviceKind}.String()
	DeviceKindAPIVersion   = DeviceKind + "." + SchemeGroupVersion.String()
	DeviceGroupVersionKind = SchemeGroupVersion.WithKind(DeviceKind)
)

func init() {
	SchemeBuilder.Register(&Device{}, &DeviceList{})
}

func (in *DeviceParameters) DeepCopyInto(out *DeviceParameters) {
	// Copia i campi primitivi o non puntatori
	*out = *in
	out.DeviceStruct = proto.Clone(in.DeviceStruct).(*api.Device)
}
