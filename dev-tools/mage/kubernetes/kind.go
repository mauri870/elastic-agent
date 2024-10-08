// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package kubernetes

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// KindIntegrationTestStep setups a kind environment.
type KindIntegrationTestStep struct{}

// Name returns the kind name.
func (m *KindIntegrationTestStep) Name() string {
	return "kind"
}

// Use always returns false.
//
// This step should be defined in `StepRequirements` for the tester, for it
// to be used. It cannot be autodiscovered for usage.
func (m *KindIntegrationTestStep) Use(dir string) (bool, error) {
	return false, nil
}

// Setup ensures that a kubernetes cluster is up and running.
//
// If `KUBECONFIG` is already deinfed in the env then it will do nothing.
func (m *KindIntegrationTestStep) Setup(env map[string]string) error {

	envVars := []string{"KUBECONFIG", "KUBE_CONFIG"}
	for _, envVar := range envVars {
		exists := envKubeConfigExists(env, envVar)
		if exists {
			return nil
		}
	}

	_, err := exec.LookPath("kind")
	if err != nil {
		if mg.Verbose() {
			fmt.Println("Skipping kind setup; kind command missing")
		}
		return nil
	}

	clusterName := kubernetesClusterName()
	stdOut := io.Discard
	stdErr := io.Discard
	if mg.Verbose() {
		stdOut = os.Stdout
		stdErr = os.Stderr
	}

	kubeCfgDir := filepath.Join("build", "kind", clusterName)
	kubeCfgDir, err = filepath.Abs(kubeCfgDir)
	if err != nil {
		return err
	}
	kubeConfig := filepath.Join(kubeCfgDir, "kubecfg")
	if mg.Verbose() {
		fmt.Println("Kubeconfig: ", kubeConfig)
	}
	if err := os.MkdirAll(kubeCfgDir, os.ModePerm); err != nil {
		return err
	}

	args := []string{
		"create",
		"cluster",
		"--name", clusterName,
		"--kubeconfig", kubeConfig,
		"--wait",
		"300s",
	}
	kubeVersion := os.Getenv("K8S_VERSION")
	if kubeVersion != "" {
		args = append(args, "--image", fmt.Sprintf("kindest/node:%s", kubeVersion))
	}

	_, err = sh.Exec(
		map[string]string{},
		stdOut,
		stdErr,
		"kind",
		args...,
	)
	if err != nil {
		return err
	}
	env["KUBECONFIG"] = kubeConfig
	env["KIND_CLUSTER"] = clusterName
	return nil
}

// Teardown destroys the kubernetes cluster.
func (m *KindIntegrationTestStep) Teardown(env map[string]string) error {
	stdOut := io.Discard
	stdErr := io.Discard
	if mg.Verbose() {
		stdOut = os.Stdout
		stdErr = os.Stderr
	}

	name, created := env["KIND_CLUSTER"]
	_, keepUp := os.LookupEnv("KIND_SKIP_DELETE")
	if created && !keepUp {
		_, err := sh.Exec(
			env,
			stdOut,
			stdErr,
			"kind",
			"delete",
			"cluster",
			"--name",
			name,
		)
		if err != nil {
			return err
		}
		delete(env, "KIND_CLUSTER")
	}
	return nil
}

func envKubeConfigExists(env map[string]string, envVar string) bool {
	_, exists := env[envVar]
	if exists {
		if mg.Verbose() {
			fmt.Printf("%s: %s\n", envVar, env[envVar])
		}
		if _, err := os.Stat(env[envVar]); err == nil {
			return true
		} else if os.IsNotExist(err) {
			if mg.Verbose() {
				fmt.Printf("%s file not found: %s: %v\n", envVar, env[envVar], err)
			}
		}
	}
	return false
}
