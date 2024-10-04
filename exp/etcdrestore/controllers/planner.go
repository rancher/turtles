/*
Copyright © 2023 - 2024 SUSE LLC

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

package controllers

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"

	bootstrapv1 "github.com/rancher/cluster-api-provider-rke2/bootstrap/api/v1beta1"
	snapshotrestorev1 "github.com/rancher/turtles/exp/etcdrestore/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Planner is responsible for executing instructions on the underlying machine host
// in the specified order, and collecting output from executed steps.
type Planner struct {
	client.Client
	machine *clusterv1.Machine
	secret  *corev1.Secret
}

// Instructions is a one time operation, used to perform shell commands on the host
type Instruction struct {
	Name       string   `json:"name,omitempty"`
	Image      string   `json:"image,omitempty"`
	Env        []string `json:"env,omitempty"`
	Args       []string `json:"args,omitempty"`
	Command    string   `json:"command,omitempty"`
	SaveOutput bool     `json:"saveOutput,omitempty"`
}

// Instructions is a list of instructions
type Instructions []Instruction

type plan struct {
	Instructions Instructions `json:"instructions,omitempty"`
}

// Plan is initializing Planner, used to perform instructions in a specific order and collect results
func Plan(ctx context.Context, c client.Client, machine *clusterv1.Machine) *Planner {
	return &Planner{
		Client:  c,
		machine: machine,
		secret:  initSecret(machine, map[string][]byte{}),
	}
}

func initSecret(machine *clusterv1.Machine, data map[string][]byte) *corev1.Secret {
	planSecretName := strings.Join([]string{machine.Spec.Bootstrap.ConfigRef.Name, "rke2config", "plan"}, "-")

	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: machine.Namespace,
			Name:      planSecretName,
		},
		Data: data,
	}
}

// Apply performs invocation of the supplied set of instructions, and reurns the ongoing state of the
// command execution
func (p *Planner) Apply(ctx context.Context, instructions ...Instruction) (Output, error) {
	var err error
	errs := []error{}

	if err := p.refresh(ctx); err != nil {
		return Output{}, err
	}

	data, err := json.Marshal(plan{Instructions: instructions})
	if err != nil {
		return Output{}, err
	}

	errs = append(errs, p.updatePlanSecret(ctx, data))
	errs = append(errs, p.refresh(ctx))

	output := Output{
		Machine:  p.machine,
		Finished: p.applied(data, p.secret.Data["applied-checksum"]),
	}

	if output.Finished {
		output.Result, err = p.Output()
		errs = append(errs, err)
	}

	return output, kerrors.NewAggregate(errs)
}

// Output holds results of the command execution
type Output struct {
	Machine  *clusterv1.Machine
	Finished bool
	Result   map[string][]byte
}

// Output retuns structured output from the command invocation
func (p *Planner) Output() (map[string][]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(p.secret.Data["applied-output"]))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	decompressedPlanOutput, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	outputMap := map[string][]byte{}
	if err := json.Unmarshal(decompressedPlanOutput, &outputMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal output: %w", err)
	}

	return outputMap, nil
}

// RKE2KillAll stops RKE2 server or agent on the node
func RKE2KillAll() Instruction {
	return Instruction{
		Name:    "shutdown",
		Command: "/bin/sh",
		Args: []string{
			"-c",
			"if [ -z $(command -v rke2) ] && [ -z $(command -v rke2-killall.sh) ]; then echo rke2 does not appear to be installed; exit 0; else rke2-killall.sh; fi",
		},
		SaveOutput: true,
	}
}

// ETCDRestore performs restore form a snapshot path on the init node
func ETCDRestore(snapshot *snapshotrestorev1.ETCDMachineSnapshot) Instruction {
	return Instruction{
		Name:    "etcd-restore",
		Command: "/bin/sh",
		Args: []string{
			"-c",
			"rke2 server --cluster-reset",
			fmt.Sprintf("--cluster-reset-restore-path=%s", strings.TrimPrefix(snapshot.Spec.Location, "file://")),
		},
		SaveOutput: true,
	}
}

// ManifestRemoval cleans up old rke2 manifests on the machine
func ManifestRemoval() Instruction {
	return Instruction{
		Name:    "remove-server-manifests",
		Command: "/bin/sh",
		Args: []string{
			"-c",
			"rm -rf /var/lib/rancher/rke2/server/manifests/rke2-*.yaml",
		},
		SaveOutput: true,
	}
}

// RemoveServerURL deletes previous server url from config, allowing nodes to register using
// new init machine
func RemoveServerURL() Instruction {
	return Instruction{
		Name:    "remove-server-manifests",
		Command: "/bin/sh",
		Args: []string{
			"-c",
			"sed -i '/^server:/d' /etc/rancher/rke2/config.yaml",
		},
		SaveOutput: true,
	}
}

// SetServerURL sets the init machine URL, used to register RKE2 agents
func SetServerURL(serverIP string) Instruction {
	return Instruction{
		Name:    "replace-server-url",
		Command: "/bin/sh",
		Args: []string{
			"-c",
			fmt.Sprintf("echo 'server: https://%s:9345' >> /etc/rancher/rke2/config.yaml", serverIP),
		},
		SaveOutput: true,
	}
}

// RemoveETCDData removes etcd snapshot state form the machine
func RemoveETCDData() Instruction {
	return Instruction{
		Name:    "remove-etcd-db-dir",
		Command: "/bin/sh",
		Args: []string{
			"-c",
			"rm -rf /var/lib/rancher/rke2/server/db/etcd",
		},
		SaveOutput: true,
	}
}

// StartRKE2 start the RKE2 service
func StartRKE2() Instruction {
	return Instruction{
		Name:    "start-rke2",
		Command: "/bin/sh",
		Args: []string{
			"-c",
			"systemctl start rke2-server.service",
		},
		SaveOutput: true,
	}
}

func (p *Planner) applied(plan, appliedChecksum []byte) bool {
	result := sha256.Sum256(plan)
	planHash := hex.EncodeToString(result[:])

	return planHash == string(appliedChecksum)
}

func (p *Planner) updatePlanSecret(ctx context.Context, data []byte) error {
	log := log.FromContext(ctx)

	if p.secret.Data == nil {
		p.secret.Data = map[string][]byte{}
	}

	if !bytes.Equal(p.secret.Data["plan"], data) {
		log.Info("Plan secret not filled with proper plan", "machine", p.machine.Name)
	}

	p.secret.Data["plan"] = []byte(data)
	p.secret = initSecret(p.machine, p.secret.Data)

	if err := p.Client.Patch(ctx, p.secret, client.Apply, []client.PatchOption{
		client.ForceOwnership,
		client.FieldOwner("etcdrestore-controller"),
	}...); err != nil {
		return fmt.Errorf("failed to patch plan secret: %w", err)
	}

	if !bytes.Equal(p.secret.Data["plan"], data) {
		log.Info("Patched plan secret with plan", "machine", p.machine.Name)
	}

	return nil
}

func (p *Planner) refresh(ctx context.Context) error {
	rke2Config := &bootstrapv1.RKE2Config{}
	if err := p.Client.Get(ctx, client.ObjectKey{
		Namespace: p.machine.Namespace,
		Name:      p.machine.Spec.Bootstrap.ConfigRef.Name,
	}, rke2Config); err != nil {
		return fmt.Errorf("failed to get RKE2Config: %w", err)
	}

	secret := &corev1.Secret{}
	if err := p.Client.Get(ctx, client.ObjectKeyFromObject(p.secret), secret); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to get plan secret: %w", err)
	} else if err == nil {
		p.secret = secret
	}

	return nil
}
