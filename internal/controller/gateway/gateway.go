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

package gateway

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/provider-chirpstack/apis/core/v1alpha1"
	apisv1alpha1 "github.com/crossplane/provider-chirpstack/apis/v1alpha1"
	"github.com/crossplane/provider-chirpstack/internal/features"

	"github.com/chirpstack/chirpstack/api/go/v4/api"
	"github.com/chirpstack/chirpstack/api/go/v4/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	errNotGateway   = "managed resource is not a Gateway custom resource"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errGetCreds     = "cannot get credentials"

	errNewClient = "cannot create new Service"
)

// Setup adds a controller that reconciles Gateway managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.GatewayGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.GatewayGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:         mgr.GetClient(),
			usage:        resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			newServiceFn: api.NewGatewayServiceClient}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.Gateway{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube         client.Client
	usage        resource.Tracker
	newServiceFn func(cc grpc.ClientConnInterface) api.GatewayServiceClient
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.Gateway)
	if !ok {
		return nil, errors.New(errNotGateway)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	pc := &apisv1alpha1.ProviderConfig{}
	if err := c.kube.Get(ctx, types.NamespacedName{Name: cr.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetPC)
	}

	cd := pc.Spec.Credentials
	data, err := resource.CommonCredentialExtractor(ctx, cd.Source, c.kube, cd.CommonCredentialSelectors)
	if err != nil {
		return nil, errors.Wrap(err, errGetCreds)
	}
	server, err := resource.CommonCredentialExtractor(ctx, pc.Spec.Host.Source, c.kube, pc.Spec.Host.CommonCredentialSelectors)
	if err != nil {
		return nil, errors.Wrap(err, errGetCreds)
	}

	dialOpts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithPerRPCCredentials(APIToken(data)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// remove this when using TLS
	}

	conn, err := grpc.Dial(string(server), dialOpts...)

	svc := c.newServiceFn(conn)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{service: svc}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	// A 'client' used to connect to the external resource API. In practice this
	// would be something like an AWS SDK client.
	service interface{}
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Gateway)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotGateway)
	}

	svc := c.service.(api.GatewayServiceClient)
	resp, err := svc.Get(ctx, &api.GetGatewayRequest{GatewayId: cr.Spec.ForProvider.GatewayId})

	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(resource.Ignore(IsErrorNotFound, err), "can not get the gateway instance")
	}

	fmt.Printf("Observing gateway: " + cr.Name + "\n")

	if !reflect.DeepEqual(toGatewayParameters(resp.Gateway), cr.Spec.ForProvider) {
		return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
	}
	return managed.ExternalObservation{
		// Return false when the external resource does not exist. This lets
		// the managed resource reconciler know that it needs to call Create to
		// (re)create the resource, or that it has successfully been deleted.
		ResourceExists: true,

		// Return false when the external resource exists, but it not up to date
		// with the desired managed resource state. This lets the managed
		// resource reconciler know that it needs to call Update.
		ResourceUpToDate: true,

		// Return any details that may be required to connect to the external
		// resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Gateway)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotGateway)
	}
	svc := c.service.(api.GatewayServiceClient)

	gateway := &api.Gateway{
		Name:          cr.Spec.ForProvider.Name,
		GatewayId:     cr.Spec.ForProvider.GatewayId,
		Description:   cr.Spec.ForProvider.Description,
		Location:      getLocationFromCr(cr.Spec.ForProvider.Location),
		Tags:          cr.Spec.ForProvider.Tags,
		Metadata:      cr.Spec.ForProvider.Metadata,
		StatsInterval: cr.Spec.ForProvider.StatsInterval,
		TenantId:      cr.Spec.ForProvider.TenantId,
	}
	_, err := svc.Create(ctx, &api.CreateGatewayRequest{Gateway: gateway})
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	fmt.Printf("Creating gateway: " + cr.Spec.ForProvider.Name + "\n")

	return managed.ExternalCreation{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Gateway)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotGateway)
	}

	fmt.Printf("Updating: " + cr.Name + "\n")

	gateway := &api.Gateway{
		Name:          cr.Spec.ForProvider.Name,
		GatewayId:     cr.Spec.ForProvider.GatewayId,
		Description:   cr.Spec.ForProvider.Description,
		Location:      getLocationFromCr(cr.Spec.ForProvider.Location),
		Tags:          cr.Spec.ForProvider.Tags,
		Metadata:      cr.Spec.ForProvider.Metadata,
		StatsInterval: cr.Spec.ForProvider.StatsInterval,
		TenantId:      cr.Spec.ForProvider.TenantId,
	}

	svc := c.service.(api.GatewayServiceClient)
	_, err := svc.Update(ctx, &api.UpdateGatewayRequest{Gateway: gateway})
	return managed.ExternalUpdate{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, err
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.Gateway)
	if !ok {
		return errors.New(errNotGateway)
	}

	fmt.Printf("Deleting: " + cr.Name + "\n")
	svc := c.service.(api.GatewayServiceClient)
	deletereq := api.DeleteGatewayRequest{GatewayId: cr.Spec.ForProvider.GatewayId}
	_, err := svc.Delete(ctx, &deletereq)
	return err
}

type APIToken string

func (a APIToken) GetRequestMetadata(ctx context.Context, url ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": fmt.Sprintf("Bearer %s", a),
	}, nil
}

func (a APIToken) RequireTransportSecurity() bool {
	return false
}

func getLocationFromCr(locIn v1alpha1.Location) *common.Location {
	lat, _ := strconv.ParseFloat(locIn.Latitude, 64)
	lon, _ := strconv.ParseFloat(locIn.Longitude, 64)
	alt, _ := strconv.ParseFloat(locIn.Altitude, 64)
	src := common.LocationSource(locIn.Source)
	acc, _ := strconv.ParseFloat(locIn.Accuracy, 32)
	return &common.Location{
		Latitude:  lat,
		Longitude: lon,
		Altitude:  alt,
		Source:    src,
		Accuracy:  float32(acc),
	}
}

func getLocationFromLoc(locIn *common.Location) *v1alpha1.Location {
	lat := strconv.FormatFloat(locIn.Latitude, 'f', -1, 64)
	lon := strconv.FormatFloat(locIn.Longitude, 'f', -1, 64)
	alt := strconv.FormatFloat(locIn.Altitude, 'f', -1, 64)
	src := locIn.Source
	acc := strconv.FormatFloat(float64(locIn.Accuracy), 'f', -1, 32)
	return &v1alpha1.Location{
		Latitude:  lat,
		Longitude: lon,
		Altitude:  alt,
		Source:    int32(src),
		Accuracy:  acc,
	}
}

// TO MOVE
func IsErrorNotFound(err error) bool {
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), "Object does not exist") {
		return true
	}
	return false

}

func toGatewayParameters(in *api.Gateway) v1alpha1.GatewayParameters {
	out := v1alpha1.GatewayParameters{
		Name:          in.Name,
		Description:   in.Description,
		TenantId:      in.TenantId,
		Tags:          in.Tags,
		GatewayId:     in.GatewayId,
		Location:      *getLocationFromLoc(in.Location),
		Metadata:      in.Metadata,
		StatsInterval: in.StatsInterval,
	}
	return out
}
