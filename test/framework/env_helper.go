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
	"reflect"
	"strings"

	"dario.cat/mergo"
	"github.com/caarlos0/env/v11"
)

var in []interface{}

var EnvOptions = env.Options{FuncMap: map[reflect.Type]env.ParserFunc{
	reflect.TypeOf(in): func(v string) (interface{}, error) {
		return Intervals(strings.Split(v, ",")), nil
	},
}}

func Parse[T any](dst *T) error {
	src := new(T)
	if err := env.ParseWithOptions(src, EnvOptions); err != nil {
		return err
	}

	return mergo.Merge(dst, src)
}

func Intervals(intervals []string) []interface{} {
	intervalsConverted := make([]interface{}, len(intervals))

	for i, v := range intervals {
		intervalsConverted[i] = v
	}

	return intervalsConverted
}
