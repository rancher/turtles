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

package webhooks

import (
	"cmp"
	"context"
	"fmt"
	"os"

	authv1 "k8s.io/api/authorization/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func validateRBAC(ctx context.Context, cl client.Client, clusterName, clusterNamespace string) error {
	admissionRequest, err := admission.RequestFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get admission request from context: %w", err)
	}

	namespace := cmp.Or(os.Getenv("POD_NAMESPACE"), "rancher-turtles-system")

	turtlesController := fmt.Sprintf("system:serviceaccount:%s:rancher-turtles-day2-operations-manager", namespace)
	if admissionRequest.UserInfo.Username == turtlesController {
		return nil
	}

	sar := authv1.SubjectAccessReview{
		Spec: authv1.SubjectAccessReviewSpec{
			ResourceAttributes: &authv1.ResourceAttributes{
				Verb:      "*",
				Group:     clusterv1.GroupVersion.Group,
				Version:   clusterv1.GroupVersion.Version,
				Resource:  "clusters",
				Name:      clusterName,
				Namespace: clusterNamespace,
			},
			User:   admissionRequest.UserInfo.Username,
			Groups: admissionRequest.UserInfo.Groups,
			UID:    admissionRequest.UserInfo.UID,
		},
	}

	if err := cl.Create(ctx, &sar); err != nil {
		return err
	}

	if !sar.Status.Allowed {
		return fmt.Errorf("user is not allowed to access the cluster: %s", sar.Status.Reason)
	}

	return nil
}
