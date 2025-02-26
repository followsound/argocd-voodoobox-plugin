package main

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func ensureBuild(ctx context.Context, cwd string, app applicationInfo) (string, error) {
	// Even when there is no git SSH secret defined, we still override the
	// git ssh command (pointing the key to /dev/null) in order to avoid
	// using ssh keys in default system locations and to surface the error
	// if bases over ssh have been configured.
	sshCmdEnv := `GIT_SSH_COMMAND=ssh -q -F none -o IdentitiesOnly=yes -o IdentityFile=/dev/null -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no`

	kFiles, err := findKustomizeFiles(cwd)
	if err != nil {
		return "", fmt.Errorf("unable to ge kustomize files paths err:%s", err)
	}

	if len(kFiles) == 0 {
		return findAndReadYamlFiles(cwd)
	}

	hasRemoteBase, err := hasSSHRemoteBaseURL(kFiles)
	if err != nil {
		return "", fmt.Errorf("unable to look for ssh protocol err:%s", err)
	}

	if hasRemoteBase {
		sshCmdEnv, err = setupGitSSH(ctx, cwd, app)
		if err != nil {
			return "", err
		}
	}

	// setup env for Kustomize command
	env := []string{
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
	}

	env = append(env, sshCmdEnv)
	// Set HOME to cwd, this means that SSH should not pick up any
	// local SSH keys and use them for cloning
	env = append(env, fmt.Sprintf("HOME=%s", cwd))

	return runKustomizeBuild(ctx, cwd, env)
}

func findKustomizeFiles(cwd string) ([]string, error) {
	kFiles := []string{}

	err := filepath.WalkDir(cwd, func(path string, info fs.DirEntry, err error) error {
		if filepath.Base(path) == "kustomization.yaml" ||
			filepath.Base(path) == "kustomization.yml" ||
			filepath.Base(path) == "Kustomization" {
			kFiles = append(kFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return kFiles, nil
}

func hasSSHRemoteBaseURL(kFiles []string) (bool, error) {
	for _, k := range kFiles {
		data, err := os.ReadFile(k)
		if err != nil {
			return false, err
		}
		if bytes.Contains(data, []byte("ssh://")) {
			return true, nil
		}
	}
	return false, nil
}

// runKustomizeBuild will run `kustomize build` cmd and return generated yaml or error
func runKustomizeBuild(ctx context.Context, cwd string, env []string) (string, error) {
	k := exec.CommandContext(ctx, "kustomize", "build", ".", "--enable-helm")

	k.Dir = cwd
	k.Env = env

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	k.Stdout = &stdout
	k.Stderr = &stderr

	if err := k.Start(); err != nil {
		return "", fmt.Errorf("unable to start kustomize cmd err:%s", err)
	}

	if err := k.Wait(); err != nil {
		return "", fmt.Errorf("error running kustomize err:%s", strings.TrimSpace(stderr.String()))
	}

	return stdout.String(), nil
}

func findAndReadYamlFiles(cwd string) (string, error) {
	var content string
	err := filepath.WalkDir(cwd, func(path string, info fs.DirEntry, err error) error {
		if filepath.Ext(path) == ".yaml" || filepath.Base(path) == ".yml" {
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("unable to read file %s err:%s", path, err)
			}
			content += fmt.Sprintf("%s\n---\n", data)
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	return content, nil
}
