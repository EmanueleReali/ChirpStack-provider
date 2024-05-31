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

	"github.com/chirpstack/chirpstack/api/go/v4/api"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"google.golang.org/protobuf/proto"

	// "google.golang.org/protobuf/proto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// DeviceProfileParameters are the configurable fields of a DeviceProfile.
type DeviceProfileParameters struct {
	DeviceProfileStruct *api.DeviceProfile `json:"device_profile"`
}

// DeviceProfileObservation are the observable fields of a DeviceProfile.
type DeviceProfileObservation struct {
	ObservableField string `json:"observableField,omitempty"`
}

// A DeviceProfileSpec defines the desired state of a DeviceProfile.
type DeviceProfileSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       DeviceProfileParameters `json:"forProvider"`
}

// A DeviceProfileStatus represents the observed state of a DeviceProfile.
type DeviceProfileStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          DeviceProfileObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A DeviceProfile is an example API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,chirpstack}
type DeviceProfile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeviceProfileSpec   `json:"spec"`
	Status DeviceProfileStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DeviceProfileList contains a list of DeviceProfile
type DeviceProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeviceProfile `json:"items"`
}

// DeviceProfile type metadata.
var (
	DeviceProfileKind             = reflect.TypeOf(DeviceProfile{}).Name()
	DeviceProfileGroupKind        = schema.GroupKind{Group: Group, Kind: DeviceProfileKind}.String()
	DeviceProfileKindAPIVersion   = DeviceProfileKind + "." + SchemeGroupVersion.String()
	DeviceProfileGroupVersionKind = SchemeGroupVersion.WithKind(DeviceProfileKind)
)

func init() {
	SchemeBuilder.Register(&DeviceProfile{}, &DeviceProfileList{})
}

func (in *DeviceProfileParameters) DeepCopyInto(out *DeviceProfileParameters) {
	// Copia i campi primitivi o non puntatori
	*out = *in
	out.DeviceProfileStruct = proto.Clone(in.DeviceProfileStruct).(*api.DeviceProfile)
}
