package sync

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	turtlesv1 "github.com/rancher-sandbox/rancher-turtles/api/v1alpha1"
)

// DefaultSyncer is a structure mirroring state of the CAPI Operator Provider object.
type DefaultSyncer struct {
	client      client.Client
	Source      *turtlesv1.CAPIProvider
	Destination client.Object
}

// NewDefaultSyncer returns a new instance of DefaultSyncer.
func NewDefaultSyncer(cl client.Client, capiProvider *turtlesv1.CAPIProvider, destination client.Object) *DefaultSyncer {
	return &DefaultSyncer{
		client:      cl,
		Source:      capiProvider,
		Destination: destination,
	}
}

// Get updates the destination object from the cluster.
func (s *DefaultSyncer) Get(ctx context.Context) error {
	log := log.FromContext(ctx)

	if err := s.client.Get(ctx, client.ObjectKeyFromObject(s.Destination), s.Destination); client.IgnoreNotFound(err) != nil {
		log.Error(err, fmt.Sprintf("Unable to get mirrored manifest: %s", client.ObjectKeyFromObject(s.Destination).String()))

		return err
	}

	return nil
}

// Apply applies the destination object to the cluster.
func (s *DefaultSyncer) Apply(ctx context.Context, reterr *error) {
	log := log.FromContext(ctx)
	uid := s.Destination.GetUID()

	setOwnerReference(s.Source, s.Destination)

	if err := Patch(ctx, s.client, s.Destination); err != nil {
		*reterr = kerrors.NewAggregate([]error{*reterr, err})
		log.Error(*reterr, fmt.Sprintf("Unable to patch object: %s", *reterr))
	}

	if s.Destination.GetUID() != uid {
		log.Info(fmt.Sprintf("Created %s: %s", s.Destination.GetObjectKind().GroupVersionKind().String(), client.ObjectKeyFromObject(s.Destination)))
	}
}

func setOwnerReference(owner, obj client.Object) {
	obj.SetFinalizers([]string{metav1.FinalizerDeleteDependents})
	obj.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion:         turtlesv1.GroupVersion.String(),
		Kind:               turtlesv1.ProviderKind,
		Name:               owner.GetName(),
		UID:                owner.GetUID(),
		Controller:         pointer.Bool(true),
		BlockOwnerDeletion: pointer.Bool(true),
	}})
}
