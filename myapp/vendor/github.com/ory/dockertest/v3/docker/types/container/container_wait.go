// Copyright © 2024 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package container

// ----------------------------------------------------------------------------
// DO NOT EDIT THIS FILE
// This file was generated by `swagger generate operation`
//
// See hack/generate-swagger-api.sh
// ----------------------------------------------------------------------------

// ContainerWaitOKBodyError container waiting error, if any
// swagger:model ContainerWaitOKBodyError
type ContainerWaitOKBodyError struct {

	// Details of an error
	Message string `json:"Message,omitempty"`
}

// ContainerWaitOKBody OK response to ContainerWait operation
// swagger:model ContainerWaitOKBody
type ContainerWaitOKBody struct {

	// error
	// Required: true
	Error *ContainerWaitOKBodyError `json:"Error"`

	// Exit code of the container
	// Required: true
	StatusCode int64 `json:"StatusCode"`
}
