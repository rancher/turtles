package sync

import (
	"context"

	turtlesv1 "github.com/rancher-sandbox/rancher-turtles/api/v1alpha1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Syncer is an inteface for mirroring state of the CAPI Operator Provider object on child objects.
type Syncer interface {
	Template(*turtlesv1.CAPIProvider) client.Object
	Get(context.Context) error
	Sync(context.Context) error
	Apply(context.Context, *error)
}

// SyncerList contains a list of syncers to apply the syncing logic.
type SyncerList []Syncer

// Sync applies synchronization logic on all syncers in the list.
func (s SyncerList) Sync(ctx context.Context) error {
	errors := []error{}
	for _, syncer := range s {
		errors = append(errors, syncer.Get(ctx), syncer.Sync(ctx))
	}

	return kerrors.NewAggregate(errors)
}

// Apply updates all syncer objects in the cluster.
func (s SyncerList) Apply(ctx context.Context, reterr *error) {
	for _, syncer := range s {
		syncer.Apply(ctx, reterr)
	}
}
