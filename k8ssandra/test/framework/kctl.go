package framework

/**
Copyright 2022 DataStax, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

 https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
**/

// TODO - sharing as module with source of this util - k8ssandra-operator
// Future will share a common library.

import (
	"bytes"
	"github.com/gruntwork-io/terratest/modules/logger"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkg/errors"
)

type ClusterInfoOptions struct {
	Options
	Namespaces []string
	OutputDirectory string
}

type Options struct {
	Namespace  string
	Context    string
	ServerSide bool
}

var logOutput = true

func LogOutput(enabled bool) {
	logOutput = enabled
}

func Apply(t *testing.T, opts Options, arg interface{}) error {
	cmd := exec.Command("kubectl")

	if len(opts.Context) > 0 {
		cmd.Args = append(cmd.Args, "--context", opts.Context)
	}

	if len(opts.Namespace) > 0 {
		cmd.Args = append(cmd.Args, "-n", opts.Namespace)
	}

	cmd.Args = append(cmd.Args, "apply")

	if opts.ServerSide {
		cmd.Args = append(cmd.Args, "--server-side", "--force-conflicts")
	}

	cmd.Args = append(cmd.Args, "-f")

	if buf, ok := arg.(*bytes.Buffer); ok {
		cmd.Stdin = buf
		cmd.Args = append(cmd.Args, "-")
	} else if s, ok := arg.(string); ok {
		cmd.Args = append(cmd.Args, s)
	} else {
		return errors.New("Expected arg to be a *bytes.Buffer or a string")
	}

	logger.Log(t, cmd)

	output, err := cmd.CombinedOutput()

	if logOutput || err != nil {
		logger.Log(t, string(output))
	}

	return err
}

func Delete(t *testing.T, opts Options, arg interface{}) error {
	cmd := exec.Command("kubectl")

	if len(opts.Context) > 0 {
		cmd.Args = append(cmd.Args, "--context", opts.Context)
	}

	if len(opts.Namespace) > 0 {
		cmd.Args = append(cmd.Args, "-n", opts.Namespace)
	}

	cmd.Args = append(cmd.Args, "delete", "-f")

	if buf, ok := arg.(*bytes.Buffer); ok {
		cmd.Stdin = buf
		cmd.Args = append(cmd.Args, "-")
	} else if s, ok := arg.(string); ok {
		cmd.Args = append(cmd.Args, s)
	} else {
		return errors.New("Expected arg to be a *bytes.Buffer or a string")
	}

	logger.Log(t, cmd)

	output, err := cmd.CombinedOutput()

	if logOutput || err != nil {
		logger.Log(t, string(output))
	}

	return err
}

func DeleteByName(t *testing.T, opts Options, kind, name string, ignoreNotFound bool) error {
	cmd := exec.Command("kubectl")
	if len(opts.Context) > 0 {
		cmd.Args = append(cmd.Args, "--context", opts.Context)
	}
	if len(opts.Namespace) > 0 {
		cmd.Args = append(cmd.Args, "-n", opts.Namespace)
	}
	cmd.Args = append(cmd.Args, "delete", kind, name)
	if ignoreNotFound {
		cmd.Args = append(cmd.Args, "--ignore-not-found")
	}
	logger.Log(t, cmd)
	output, err := cmd.CombinedOutput()
	if logOutput || err != nil {
		logger.Log(t, string(output))
	}
	return err
}

func DeleteAllOf(t *testing.T, opts Options, kind string) error {
	cmd := exec.Command("kubectl")
	if len(opts.Context) > 0 {
		cmd.Args = append(cmd.Args, "--context", opts.Context)
	}

	if len(opts.Namespace) > 0 {
		cmd.Args = append(cmd.Args, "-n", opts.Namespace)
	} else {
		return errors.New("Namespace is required for delete --all")
	}
	cmd.Args = append(cmd.Args, "delete", kind, "--all")

	logger.Log(t, cmd)

	output, err := cmd.CombinedOutput()
	if logOutput || err != nil {
		logger.Log(t, string(output))
	}
	return err
}

func WaitForCondition(t *testing.T, condition string, args ...string) error {
	kargs := []string{"wait", "--for", "condition=" + condition}
	kargs = append(kargs, args...)

	cmd := exec.Command("kubectl", kargs...)
	output, err := cmd.CombinedOutput()

	if logOutput || err != nil {
		logger.Log(t, string(output))
	}
	return err
}

// Exec executes a command against a Cassandra pod and the cassandra container in
// particular. This does not currently handle pipes.
func Exec(t *testing.T, opts Options, pod string, args ...string) (string, error) {
	cmd := exec.Command("kubectl")

	if len(opts.Context) > 0 {
		cmd.Args = append(cmd.Args, "--context", opts.Context)
	}

	if len(opts.Namespace) > 0 {
		cmd.Args = append(cmd.Args, "-n", opts.Namespace)
	}

	cmd.Args = append(cmd.Args, "exec", "-i", pod, "-c", "cassandra", "--")
	cmd.Args = append(cmd.Args, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	logger.Log(t, cmd)

	err := cmd.Run()
	if err != nil {
		return stderr.String(), err
	}

	return stdout.String(), nil
}

func DumpClusterInfo(t *testing.T, opts ClusterInfoOptions) error {
	cmd := exec.Command("kubectl", "cluster-info", "dump")

	if len(opts.Context) > 0 {
		cmd.Args = append(cmd.Args, "--context", opts.Context)
	}

	if len(opts.Namespaces) > 0 {
		cmd.Args = append(cmd.Args, "--namespaces", strings.Join(opts.Namespaces, ","))
	}

	if len(opts.Namespace) > 0 {
		cmd.Args = append(cmd.Args, "-n", opts.Namespace)
	}

	cmd.Args = append(cmd.Args, "-o", "yaml")

	dir, err := filepath.Abs(opts.OutputDirectory)
	if err != nil {
		return err
	}

	cmd.Args = append(cmd.Args, "--output-directory", dir)

	output, err := cmd.CombinedOutput()

	if logOutput || err != nil {
		logger.Log(t, string(output))
	}

	return err
}
