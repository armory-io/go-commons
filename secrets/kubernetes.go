/*
 * Copyright 2022 Armory, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package secrets

import (
	"context"
	"fmt"
	"golang.org/x/exp/maps"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"os"
	"strings"
)

const (
	K8sSecretName                              = "n"
	K8sSecretKey                               = "k"
	K8sSecretNamespace                         = "ns"
	K8sGenericMalformedKeyError                = "secret format error - malformed params, expected format '[encrypted|encryptedFile]:secrets:k8s!n:some-secret-name!k:some-secret-key' or [encrypted|encryptedFile]:secrets:k8s!ns:some-namespace!n:some-secret-name!k:some-secret-key"
	K8sSecretNameMissingError                  = "secret name `!n:` parameter is missing ex: encrypted:k8s!n:kubernetes-secret-name!k:secret-key"
	K8sSecretKeyMissingError                   = "secret key `!k:` parameter is missing ex: encrypted:k8s!n:kubernetes-secret-name!k:secret-key"
	K8sUnableToDetermineNameSpaceErrorTemplate = "failed to determine namespace, you must supply the `!ns:` key or be running on a pod where %s is defined"
	K8sSecretHasNoDataError                    = "the secret %s has no data"
	K8sNamespaceFilePath                       = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	K8sFailedToConfigureClientErrorTemplate    = "failed to create k8s client config, to fetch secret: %s, err: %s"
	K8sKeyNotFoundErrorTemplate                = "the key %s was not found or was empty in secret %s, keys present [%s]"
)

type KubernetesDecrypter struct {
	rawToken        string
	secretName      string
	secretKey       string
	secretNamespace string
	isFile          bool
	ctx             context.Context
}

func NewKubernetesSecretDecrypter(ctx context.Context, isFile bool, params string) (Decrypter, error) {
	kubernetesDecrypter := &KubernetesDecrypter{isFile: isFile, ctx: ctx}
	if err := kubernetesDecrypter.parse(params); err != nil {
		return nil, err
	}

	return kubernetesDecrypter, nil
}

func (k *KubernetesDecrypter) Decrypt() (string, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return "", fmt.Errorf(K8sFailedToConfigureClientErrorTemplate, k.rawToken, err.Error())
	}
	k8s, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", fmt.Errorf(K8sFailedToConfigureClientErrorTemplate, k.rawToken, err.Error())
	}
	secret, err := k8s.CoreV1().Secrets(k.secretNamespace).Get(k.ctx, k.secretName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	data := secret.Data
	if data == nil || len(data) < 1 {
		return "", fmt.Errorf(K8sSecretHasNoDataError, k.rawToken)
	}

	bytes := data[k.secretKey]
	if bytes == nil || len(bytes) < 1 {
		return "", fmt.Errorf(K8sKeyNotFoundErrorTemplate, k.secretKey, k.rawToken, strings.Join(maps.Keys(data), ", "))
	}

	if k.isFile {
		return ToTempFile(bytes)
	}

	return string(bytes), nil
}

func (k *KubernetesDecrypter) IsFile() bool {
	return k.isFile
}

func (k *KubernetesDecrypter) parse(params string) error {
	k.rawToken = params
	// Parse the user supplied params to get a map of the k,v pairs
	var data = make(map[string]string)
	entries := strings.Split(params, "!")
	for _, entry := range entries {
		kvPair := strings.SplitN(entry, ":", 2)
		if len(kvPair) != 2 {
			return fmt.Errorf(K8sGenericMalformedKeyError)
		}
		data[kvPair[0]] = kvPair[1]
	}

	// Validate and save the required / known fields
	if secretName := data[K8sSecretName]; secretName != "" {
		k.secretName = secretName
		delete(data, K8sSecretName)
	} else {
		return fmt.Errorf(K8sSecretNameMissingError)
	}

	if secretKey := data[K8sSecretKey]; secretKey != "" {
		k.secretKey = secretKey
		delete(data, K8sSecretKey)
	} else {
		return fmt.Errorf(K8sSecretKeyMissingError)
	}

	if secretNamespace := data[K8sSecretNamespace]; secretNamespace != "" {
		k.secretNamespace = secretNamespace
		delete(data, K8sSecretNamespace)
	} else {
		nsBytes, err := os.ReadFile(K8sNamespaceFilePath)
		if err == nil {
			k.secretNamespace = strings.TrimSpace(string(nsBytes))
		}
	}
	if k.secretNamespace == "" {
		return fmt.Errorf(K8sUnableToDetermineNameSpaceErrorTemplate, K8sNamespaceFilePath)
	}

	if len(data) > 0 {
		return fmt.Errorf(K8sGenericMalformedKeyError)
	}

	return nil
}
