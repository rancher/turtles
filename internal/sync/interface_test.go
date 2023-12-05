package sync_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	turtlesv1 "github.com/rancher-sandbox/rancher-turtles/api/v1alpha1"
	"github.com/rancher-sandbox/rancher-turtles/internal/sync"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MockSynchronizer struct {
	syncronizerr error
	getErr       error
	applyErr     error
}

func (m *MockSynchronizer) Get(ctx context.Context) error {
	return m.getErr
}

func (m *MockSynchronizer) Sync(ctx context.Context) error {
	return m.syncronizerr
}

func (m *MockSynchronizer) Apply(ctx context.Context, reterr *error) {
	*reterr = m.applyErr
}

func (m *MockSynchronizer) Template(_ *turtlesv1.CAPIProvider) client.Object {
	return nil
}

var _ = Describe("resource Sync interface", func() {
	It("should synchronize a list of manifests", func() {
		testCases := []struct {
			name     string
			list     sync.List
			expected bool
			err      string
		}{
			{
				name:     "Nil syncronizer is accepted",
				list:     sync.List{nil},
				expected: false,
			}, {
				name:     "All syncronizers is executed",
				list:     sync.List{&MockSynchronizer{}, &MockSynchronizer{}},
				expected: false,
			}, {
				name:     "Syncronizer errors are returned",
				list:     sync.List{&MockSynchronizer{getErr: errors.New("Fail first get"), syncronizerr: errors.New("Fail sync")}, &MockSynchronizer{getErr: errors.New("Fail second get")}},
				err:      "Fail first get, Fail sync, Fail second get",
				expected: true,
			},
		}

		for _, tc := range testCases {
			err := tc.list.Sync(context.Background())
			Expect(tc.expected).To(Equal(err != nil), tc.name)
			if tc.err != "" {
				Expect(err.Error()).To(ContainSubstring(tc.err), tc.name)
			}
		}
	})

	It("should apply a list of manifests", func() {
		testCases := []struct {
			name     string
			list     sync.List
			expected bool
			err      string
		}{
			{
				name:     "Nil syncronizer is accepted",
				list:     sync.List{nil},
				expected: false,
			}, {
				name:     "All syncronizers is executed",
				list:     sync.List{&MockSynchronizer{}, &MockSynchronizer{}},
				expected: false,
			}, {
				name:     "Syncronizer errors are returned",
				list:     sync.List{&MockSynchronizer{applyErr: errors.New("Fail apply")}, &MockSynchronizer{applyErr: errors.New("Fail second apply")}},
				err:      "Fail apply, Fail second apply",
				expected: true,
			},
		}

		for _, tc := range testCases {
			var err error
			tc.list.Apply(context.Background(), &err)
			Expect(tc.expected).To(Equal(err != nil), tc.name)
			if tc.err != "" {
				Expect(err.Error()).To(ContainSubstring(tc.err), tc.name)
			}
		}
	})
})
