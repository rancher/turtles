package helpers

import (
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestBuildReturnsEnvtestStartError(t *testing.T) {
	t.Setenv("KUBEBUILDER_ASSETS", "")

	cfg := &TestEnvironmentConfiguration{
		env: &envtest.Environment{
			BinaryAssetsDirectory: t.TempDir(),
		},
	}

	env, err := cfg.Build()
	if err == nil {
		if env != nil {
			_ = env.Stop()
		}
		t.Fatal("expected Build to return an error when envtest assets are missing")
	}
}

func TestStopHandlesNilEnvironment(t *testing.T) {
	var testEnv *TestEnvironment

	if err := testEnv.Stop(); err != nil {
		t.Fatalf("expected nil environment stop to succeed, got %v", err)
	}
}

func TestStopHandlesPartiallyInitializedEnvironment(t *testing.T) {
	testEnv := &TestEnvironment{}

	if err := testEnv.Stop(); err != nil {
		t.Fatalf("expected partially initialized environment stop to succeed, got %v", err)
	}
}
