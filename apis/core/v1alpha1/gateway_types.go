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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// GatewayParameters are the configurable fields of a Gateway.
type GatewayParameters struct {
	// Gateway ID (EUI64).
	GatewayId string `json:"gateway_id"`
	// Name.
	Name string `json:"name"`
	// Description.
	Description string `json:"description,omitempty"`
	// Gateway location.
	Location Location `json:"location"`
	// Tenant ID (UUID).
	TenantId string `json:"tenant_id"`
	// Tags.
	Tags map[string]string `json:"tags,omitempty"`
	// Metadata (provided by the gateway).
	Metadata map[string]string `json:"metadata,omitempty"`
	// Stats interval (seconds).
	// This defines the expected interval in which the gateway sends its
	// statistics.
	StatsInterval uint32 `json:"stats_interval"`
}

type Location struct {

	// Latitude.
	Latitude string `json:"latitude,omitempty"`
	// Longitude.
	Longitude string `json:"longitude,omitempty"`
	// Altitude.
	Altitude string `json:"altitude,omitempty"`
	// Location source.
	Source int32 `json:"source,omitempty"`
	// Accuracy.
	Accuracy string `json:"accuracy,omitempty"`
}

// GatewayObservation are the observable fields of a Gateway.
type GatewayObservation struct {
	ObservableField string `json:"observableField,omitempty"`
}

// A GatewaySpec defines the desired state of a Gateway.
type GatewaySpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       GatewayParameters `json:"forProvider"`
}

// A GatewayStatus represents the observed state of a Gateway.
type GatewayStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          GatewayObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Gateway is an example API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,chirpstack}
type Gateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GatewaySpec   `json:"spec"`
	Status GatewayStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GatewayList contains a list of Gateway
type GatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Gateway `json:"items"`
}

// Gateway type metadata.
var (
	GatewayKind             = reflect.TypeOf(Gateway{}).Name()
	GatewayGroupKind        = schema.GroupKind{Group: Group, Kind: GatewayKind}.String()
	GatewayKindAPIVersion   = GatewayKind + "." + SchemeGroupVersion.String()
	GatewayGroupVersionKind = SchemeGroupVersion.WithKind(GatewayKind)
)

func init() {
	SchemeBuilder.Register(&Gateway{}, &GatewayList{})
}
