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
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	yamlDecoder "k8s.io/apimachinery/pkg/util/yaml"
)

func setTemplateParams(template string, params map[string]string) string {
	for k, v := range params {
		template = strings.ReplaceAll(template, k, v)
	}

	return template
}

func manifestToObjects(in io.Reader) ([]runtime.Object, error) {
	var result []runtime.Object

	reader := yamlDecoder.NewYAMLReader(bufio.NewReaderSize(in, 4096))

	for {
		raw, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, err
		}

		bytes, err := yamlDecoder.ToJSON(raw)
		if err != nil {
			return nil, err
		}

		check := map[string]interface{}{}
		if err := json.Unmarshal(bytes, &check); err != nil {
			return nil, err
		}

		if len(check) == 0 {
			continue
		}

		obj, _, err := unstructured.UnstructuredJSONScheme.Decode(bytes, nil, nil)
		if err != nil {
			return nil, err
		}

		result = append(result, obj)
	}

	return result, nil
}
