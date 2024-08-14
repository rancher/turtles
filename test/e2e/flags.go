//go:build e2e
// +build e2e

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

package e2e

import (
	"flag"
)

type FlagValues struct {
	// ConfigPath is the path to the e2e config file.
	ConfigPath string
}

// InitFlags is used to specify the standard flags for the e2e tests.
func InitFlags(values *FlagValues) {
	flag.StringVar(&values.ConfigPath, "e2e.config", "config/operator.yaml", "path to the e2e config file")
}
