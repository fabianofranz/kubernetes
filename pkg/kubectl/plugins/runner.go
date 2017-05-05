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
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/golang/glog"
)

// PluginRunner is capable of running a plugin in a given running context.
type PluginRunner interface {
	Run(plugin *Plugin, ctx RunningContext) error
}

// RunningContext holds the context in which a given plugin is running - the
// in, out, and err streams, arguments and environment passed to it, and the
// working directory.
type RunningContext struct {
	In          io.Reader
	Out         io.Writer
	ErrOut      io.Writer
	Args        []string
	EnvProvider RunningEnvProvider
	WorkingDir  string
}

// ExecPluginRunner is a PluginRunner that uses Go's os/exec to run plugins.
type ExecPluginRunner struct{}

// Run takes a given plugin and runs it in a given context using os/exec, returning
// any error found while running.
func (r *ExecPluginRunner) Run(plugin *Plugin, ctx RunningContext) error {
	command := strings.Split(os.ExpandEnv(plugin.Command), " ")
	base := command[0]
	args := []string{}
	if len(command) > 1 {
		args = command[1:]
	}
	args = append(args, ctx.Args...)

	cmd := exec.Command(base, args...)

	cmd.Stdin = ctx.In
	cmd.Stdout = ctx.Out
	cmd.Stderr = ctx.ErrOut

	env, err := ctx.EnvProvider.Env()
	if err != nil {
		return err
	}
	cmd.Env = env
	cmd.Dir = ctx.WorkingDir

	glog.V(9).Infof("Running plugin %q as base command %q with args %v", plugin.Name, base, args)
	return cmd.Run()
}

// RunningEnvProvider provides the environment (with entries in the KEY=VALUE form)
// in which the plugin will run.
type RunningEnvProvider interface {
	Env() ([]string, error)
}

// MultiRunningEnvProvider is a RunningEnvProvider for multiple env providers, returns on first error.
type MultiRunningEnvProvider []RunningEnvProvider

func (p MultiRunningEnvProvider) Env() ([]string, error) {
	env := []string{}
	for _, provider := range p {
		pEnv, err := provider.Env()
		if err != nil {
			return []string{}, err
		}
		env = append(env, pEnv...)
	}
	return env, nil
}

// PluginCallerEnvProvider provides env with the path to the caller binary (usually full path to 'kubectl').
type PluginCallerEnvProvider struct{}

func (p *PluginCallerEnvProvider) Env() ([]string, error) {
	caller, err := os.Executable()
	if err != nil {
		return []string{}, err
	}
	return []string{fmt.Sprintf("%s=%s", "KUBECTL_PLUGINS_CALLER", caller)}, nil
}

// PluginDescriptorEnvProvider provides env vars with information about the running plugin.
type PluginDescriptorEnvProvider struct {
	Plugin *Plugin
}

func (p *PluginDescriptorEnvProvider) Env() ([]string, error) {
	if p.Plugin == nil {
		return []string{}, fmt.Errorf("plugin not present to extract env")
	}
	prefix := "KUBECTL_PLUGINS_DESCRIPTOR_"
	env := []string{}
	env = append(env, FieldToEnv("Name", p.Plugin.Name, prefix))
	env = append(env, FieldToEnv("ShortDesc", p.Plugin.ShortDesc, prefix))
	env = append(env, FieldToEnv("LongDesc", p.Plugin.LongDesc, prefix))
	env = append(env, FieldToEnv("Example", p.Plugin.Example, prefix))
	env = append(env, FieldToEnv("Command", p.Plugin.Command, prefix))
	return env, nil
}

// OSEnvProvider provides current environment from the operating system.
type OSEnvProvider struct{}

func (p *OSEnvProvider) Env() ([]string, error) {
	return os.Environ(), nil
}

type EmptyEnvProvider struct{}

func (p *EmptyEnvProvider) Env() ([]string, error) {
	return []string{}, nil
}
