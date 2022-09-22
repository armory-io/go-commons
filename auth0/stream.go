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

package auth0

import (
    "github.com/auth0/go-auth0/management"
    "reflect"
)

type (
    hasNexter interface {
        HasNext() bool
    }

    lister[T hasNexter] interface {
        List(opts ...management.RequestOption) (*T, error)
    }
)

// Stream iterates over every member of an Auth0 API resource.
// Type parameter T should be the ___List type of the resource (e.g., UserList, ClientList).
// Type parameter U should be the base type of the resource (e.g., User, Client).
// Stream is not quite typesafe because there is no concrete relationship between the list and base type.
func Stream[T hasNexter, U any](client lister[T], fn func(val *U) error, opts ...management.RequestOption) error {
    var page int
    for {
        listResponse, err := client.List(append(opts, management.Page(page))...)
        if err != nil {
            return err
        }

        listResponseVal := reflect.ValueOf(listResponse).Elem()
        var found bool

        for i := 0; i < listResponseVal.NumField(); i++ {
            field := listResponseVal.Field(i)

            if field.Kind() == reflect.Slice {
                list, ok := field.Interface().([]*U)
                if !ok {
                    return ErrInvalidListType
                }

                for _, element := range list {
                    if err := fn(element); err != nil {
                        return err
                    }
                }
                found = true
                break
            }
        }

        if !found {
            return ErrInvalidListType
        }

        if (*listResponse).HasNext() {
            page++
        } else {
            break
        }
    }
    return nil
}
