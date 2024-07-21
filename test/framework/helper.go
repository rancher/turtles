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

package framework

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Byf is used to provider better output for a test using a formatted string.
func Byf(format string, a ...interface{}) {
	By(fmt.Sprintf(format, a...))
}

// VariableCollection represents a collection of variables for tests.
type VariableCollection map[string]string

// VariableLookupFunc is a function type used for looking up variable values.
type VariableLookupFunc func(key string) string

// GetVariable is used to get the value for a variable. The expectation is that the variable exists in one of
// the sources. Assertion will fail if its not found. The order of precedence when checking for variables is:
// 1. Environment variables
// 2. Base variables
// This is a re-implementation of the CAPI function to add additional logging.
func GetVariable(vars VariableCollection) VariableLookupFunc {
	Expect(vars).ToNot(BeNil(), "Variable should not be nil")

	return func(varName string) string {
		if value, ok := os.LookupEnv(varName); ok {
			return value
		}

		value, ok := vars[varName]
		Expect(ok).To(BeTrue(), "Could not find value for variable: %s", varName)

		return value
	}
}
