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

package application

import (
	"context"
	"fmt"
	"reflect"
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

	"github.com/chirpstack/chirpstack/api/go/v4/api"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/provider-chirpstack/apis/core/v1alpha1"
	apisv1alpha1 "github.com/crossplane/provider-chirpstack/apis/v1alpha1"
	"github.com/crossplane/provider-chirpstack/internal/features"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	errNotApplication = "managed resource is not a Application custom resource"
	errTrackPCUsage   = "cannot track ProviderConfig usage"
	errGetPC          = "cannot get ProviderConfig"
	errGetCreds       = "cannot get credentials"

	errNewClient = "cannot create new Service"
)

// Setup adds a controller that reconciles Application managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.ApplicationGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	r := managed.NewReconciler(
		mgr,
		resource.ManagedKind(v1alpha1.ApplicationGroupVersionKind),
		managed.WithExternalConnecter(&connecter{
			kube:      mgr.GetClient(),
			usage:     resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			newClient: api.NewApplicationServiceClient,
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.Application{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connecter struct {
	kube      client.Client
	usage     resource.Tracker
	newClient func(cc grpc.ClientConnInterface) api.ApplicationServiceClient
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connecter) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.Application)
	if !ok {
		return nil, errors.New(errNotApplication)
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
	server := pc.Spec.Host
	if err != nil {
		return nil, errors.Wrap(err, errGetCreds)
	}

	dialOpts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithPerRPCCredentials(APIToken(data)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// remove this when using TLS
	}
	conn, err := grpc.Dial(server, dialOpts...)
	svc := c.newClient(conn)
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
	fmt.Printf("funzione observe chiamata\n")
	cr, ok := mg.(*v1alpha1.Application)

	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotApplication)
	}

	annotations := cr.GetAnnotations()
	var id string
	for key, value := range annotations {
		if key == "id" {
			id = value
			break
		}
	}
	if id == "" {
		return managed.ExternalObservation{}, nil
	}

	svc := c.service.(api.ApplicationServiceClient)
	resp, err := svc.Get(ctx, &api.GetApplicationRequest{
		Id: id,
	})

	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(resource.Ignore(IsErrorNotFound, err), "can not get the application instance")
	}

	fmt.Printf("Observing: application: " + cr.Name + " with id: " + id + "\n")

	if !reflect.DeepEqual(toApplicationParameters(resp.Application), cr.Spec.ForProvider) {
		return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
	}
	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	fmt.Printf("funzione create chiamata\n")
	cr, ok := mg.(*v1alpha1.Application)

	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotApplication)
	}

	svc := c.service.(api.ApplicationServiceClient)
	app := &api.Application{
		Name:        cr.Spec.ForProvider.Name,
		TenantId:    cr.Spec.ForProvider.TenantId,
		Description: cr.Spec.ForProvider.Description,
		Tags:        cr.Spec.ForProvider.Tags,
	}
	req := &api.CreateApplicationRequest{Application: app}
	resp, err := svc.Create(ctx, req)
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	fmt.Printf("Creating application: " + cr.Name + " with Id: " + resp.Id + "\n")
	meta.AddAnnotations(cr, map[string]string{"id": resp.Id})
	return managed.ExternalCreation{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	fmt.Printf("funzione update chiamata\n")
	cr, ok := mg.(*v1alpha1.Application)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotApplication)
	}

	fmt.Printf("Updating:  " + cr.Name + " with id: " + cr.GetAnnotations()["id"] + "\n")

	app := api.Application{
		Name:        cr.Spec.ForProvider.Name,
		Description: cr.Spec.ForProvider.Description,
		TenantId:    cr.Spec.ForProvider.TenantId,
		Tags:        cr.Spec.ForProvider.Tags,
		Id:          cr.GetAnnotations()["id"],
	}

	svc := c.service.(api.ApplicationServiceClient)
	_, err := svc.Update(ctx, &api.UpdateApplicationRequest{Application: &app})
	return managed.ExternalUpdate{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, err
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	fmt.Printf("funzione delete chiamata\n")
	cr, ok := mg.(*v1alpha1.Application)
	if !ok {
		return errors.New(errNotApplication)
	}

	fmt.Printf("Deleting: " + cr.Name + "\n")
	svc := c.service.(api.ApplicationServiceClient)
	deletereq := api.DeleteApplicationRequest{Id: cr.GetAnnotations()["id"]}
	_, err := svc.Delete(ctx, &deletereq)
	return err
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

func toApplicationParameters(in *api.Application) v1alpha1.ApplicationParameters {
	out := v1alpha1.ApplicationParameters{
		Name:        in.Name,
		Description: in.Description,
		TenantId:    in.TenantId,
		Tags:        in.Tags,
	}
	return out
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
