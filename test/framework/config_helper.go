/*
Copyright © 2023 - 2025 SUSE LLC

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

package framework

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/yaml"

	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

func LoadE2EConfig(configPath string) *clusterctl.E2EConfig {
	Expect(configPath).To(BeAnExistingFile(), "Invalid test suite argument. e2e.config should be an existing file.")

	configData, err := os.ReadFile(configPath)
	Expect(err).ToNot(HaveOccurred(), "Failed to read the e2e test config file")
	Expect(configData).ToNot(BeEmpty(), "The e2e test config file should not be empty")

	config := &clusterctl.E2EConfig{}
	Expect(yaml.UnmarshalStrict(configData, config)).To(Succeed(), "Failed to convert the e2e test config file to yaml")

	config.Defaults()
	config.AbsPaths(filepath.Dir(configPath))

	replaceVars := []string{}
	for k, v := range config.Variables {
		if os.Getenv(k) == "" {
			Expect(os.Setenv(k, v)).To(Succeed(), "Failed to set default env value")
		}
		replaceVars = append(replaceVars, fmt.Sprintf("{%s}", k), os.Getenv(k))
	}

	imageReplacer := strings.NewReplacer(replaceVars...)
	for i := range config.Images {
		containerImage := &config.Images[i]
		containerImage.Name = imageReplacer.Replace(containerImage.Name)
	}

	return config
}
