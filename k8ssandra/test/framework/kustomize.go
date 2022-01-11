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

package framework

import (
	"bytes"
	"fmt"
	"os/exec"
)

func BuildDir(dir string) (*bytes.Buffer, error) {
	cmd := exec.Command("kustomize", "build")
	cmd.Dir = dir

	fmt.Println(cmd)

	output, err := cmd.CombinedOutput()
	buffer := bytes.NewBuffer(output)

	if logOutput {
		fmt.Println(string(output))
	}

	return buffer, err
}

func BuildUrl(url string) (*bytes.Buffer, error) {
	cmd := exec.Command("kustomize", "build", url)

	fmt.Println(cmd)

	output, err := cmd.CombinedOutput()
	buffer := bytes.NewBuffer(output)

	if logOutput {
		fmt.Println(string(output))
	}

	return buffer, err
}

func SetNamespace(dir, namespace string) error {
	cmd := exec.Command("kustomize", "edit", "set", "namespace", namespace)
	cmd.Dir = dir

	fmt.Println(cmd)

	output, err := cmd.CombinedOutput()

	if logOutput {
		fmt.Println(string(output))
	}

	return err
}

func AddResource(path string) error {
	cmd := exec.Command("kustomize", "edit", "add", "resource", path)
	cmd.Dir = "../testdata/k8ssandra-operator"

	fmt.Println(cmd)

	output, err := cmd.CombinedOutput()

	if logOutput {
		fmt.Println(string(output))
	}

	return err
}

func RemoveResource(path string) error {
	cmd := exec.Command("kustomize", "edit", "remove", "resource", path)
	cmd.Dir = "../testdata/k8ssandra-operator"

	fmt.Println(cmd)

	output, err := cmd.CombinedOutput()

	if logOutput {
		fmt.Println(string(output))
	}

	return err
}
