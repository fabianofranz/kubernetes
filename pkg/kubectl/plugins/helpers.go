/*
Copyright 2017 The Kubernetes Authors.

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

package plugins

import (
	"fmt"
	"strings"

	"github.com/fatih/camelcase"
	"github.com/spf13/pflag"
)

func FlagToEnvName(flagName, prefix string) string {
	envName := strings.TrimPrefix(flagName, "--")
	envName = strings.ToUpper(envName)
	envName = strings.Replace(envName, "-", "_", -1)
	envName = prefix + envName
	return envName
}

func FlagToEnv(flag *pflag.Flag, prefix string) string {
	envName := FlagToEnvName(flag.Name, prefix)
	return fmt.Sprintf("%s=%s", envName, flag.Value.String())
}

func FieldToEnvName(fieldName, prefix string) string {
	parts := []string{}
	for _, s := range strings.Split(fieldName, ".") {
		split := camelcase.Split(s)
		envPart := strings.Join(split, "_")
		envPart = strings.ToUpper(envPart)
		parts = append(parts, envPart)
	}
	return prefix + strings.Join(parts, "_")
}

func FieldToEnv(fieldName, fieldValue, prefix string) string {
	envName := FieldToEnvName(fieldName, prefix)
	return fmt.Sprintf("%s=%s", envName, fieldValue)
}
