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
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
)

type AwsSecretsManagerClient interface {
	FetchSecret(secretName string) (*secretsmanager.GetSecretValueOutput, error)
}

type AwsSecretsManagerClientImpl struct {
	secretsManager *secretsmanager.SecretsManager
}

func NewAwsSecretsManagerClient(region string) (AwsSecretsManagerClient, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return nil, err
	}
	secretsManager := secretsmanager.New(sess)
	client := &AwsSecretsManagerClientImpl{
		secretsManager: secretsManager,
	}
	return client, nil
}

func (a *AwsSecretsManagerClientImpl) FetchSecret(secretName string) (*secretsmanager.GetSecretValueOutput, error) {
	request := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	}

	result, err := a.secretsManager.GetSecretValue(request)

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			code := aerr.Code()
			return nil, fmt.Errorf("failed to fetch secret from AWS SM, Code: %v, Err: %v", code, aerr.Error())
		} else {
			return nil, fmt.Errorf("failed to fetch secret from AWS SM, Err: %v", err.Error())
		}
	}

	return result, nil
}
