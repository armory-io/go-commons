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

import "context"

func NewNoopDecrypter(ctx context.Context, isFile bool, params string) (Decrypter, error) {
	return &NoopDecrypter{
		value:  params,
		isFile: isFile,
	}, nil
}

type NoopDecrypter struct {
	value  string
	isFile bool
}

func (n *NoopDecrypter) Decrypt() (string, error) {
	if n.isFile {
		return ToTempFile([]byte(n.value))
	}
	return n.value, nil
}

func (n *NoopDecrypter) ParseTokens(secret string) {
	n.value = secret[len("encrypted:noop!v:"):]
}

func (n *NoopDecrypter) IsFile() bool {
	return n.isFile
}
