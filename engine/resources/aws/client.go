// Mgmt
// Copyright (C) James Shubin and the project contributors
// Written by James Shubin <james@shubin.ca> and the project contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
//
// Additional permission under GNU GPL version 3 section 7
//
// If you modify this program, or any covered work, by linking or combining it
// with embedded mcl code and modules (and that the embedded mcl code and
// modules which link with this program, contain a copy of their source code in
// the authoritative form) containing parts covered by the terms of any other
// license, the licensors of this program grant you additional permission to
// convey the resulting work. Furthermore, the licensors of this program grant
// the original author, James Shubin, additional permission to update this
// additional permission if he deems it necessary to achieve the goals of this
// additional permission.

//go:build !noaws

package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudcontrol"
	cctypes "github.com/aws/aws-sdk-go-v2/service/cloudcontrol/types"
)

const (
	// WaitPollInterval is how long to wait between polling for async
	// operation status from the Cloud Control API.
	WaitPollInterval = 5 * time.Second

	// WaitTimeout is the maximum amount of time to wait for an async
	// Cloud Control API operation to complete.
	WaitTimeout = 10 * time.Minute
)

// Client wraps the AWS Cloud Control API client with session management.
type Client struct {
	cc     *cloudcontrol.Client
	region string
}

// NewClient creates a new Cloud Control client for the given AWS region. It
// uses the standard AWS credential chain: environment variables,
// ~/.aws/credentials, IAM role, etc.
func NewClient(ctx context.Context, region string) (*Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("error loading AWS config: %w", err)
	}

	cc := cloudcontrol.NewFromConfig(cfg)

	return &Client{
		cc:     cc,
		region: region,
	}, nil
}

// Get retrieves the current state of a resource. It returns nil if the resource
// does not exist (rather than an error).
func (obj *Client) Get(ctx context.Context, typeName, identifier string) (map[string]interface{}, error) {
	input := &cloudcontrol.GetResourceInput{
		TypeName:   aws.String(typeName),
		Identifier: aws.String(identifier),
	}

	output, err := obj.cc.GetResource(ctx, input)
	if err != nil {
		// If the resource is not found, return nil.
		if isNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error getting resource %s/%s: %w", typeName, identifier, err)
	}

	if output.ResourceDescription == nil || output.ResourceDescription.Properties == nil {
		return nil, nil
	}

	var properties map[string]interface{}
	if err := json.Unmarshal([]byte(*output.ResourceDescription.Properties), &properties); err != nil {
		return nil, fmt.Errorf("error unmarshaling properties: %w", err)
	}

	return properties, nil
}

// Create creates a new resource with the given desired state. It returns the
// identifier of the created resource.
func (obj *Client) Create(ctx context.Context, typeName string, desiredState map[string]interface{}) (string, error) {
	stateJSON, err := json.Marshal(desiredState)
	if err != nil {
		return "", fmt.Errorf("error marshaling desired state: %w", err)
	}

	input := &cloudcontrol.CreateResourceInput{
		TypeName:     aws.String(typeName),
		DesiredState: aws.String(string(stateJSON)),
	}

	output, err := obj.cc.CreateResource(ctx, input)
	if err != nil {
		return "", fmt.Errorf("error creating resource %s: %w", typeName, err)
	}

	if err := obj.waitForOperation(ctx, output.ProgressEvent); err != nil {
		return "", fmt.Errorf("error waiting for create of %s: %w", typeName, err)
	}

	if output.ProgressEvent != nil && output.ProgressEvent.Identifier != nil {
		return *output.ProgressEvent.Identifier, nil
	}

	return "", nil
}

// Update applies a JSON Patch to an existing resource.
func (obj *Client) Update(ctx context.Context, typeName, identifier string, patch []PatchOp) error {
	patchJSON, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("error marshaling patch: %w", err)
	}

	input := &cloudcontrol.UpdateResourceInput{
		TypeName:    aws.String(typeName),
		Identifier:  aws.String(identifier),
		PatchDocument: aws.String(string(patchJSON)),
	}

	output, err := obj.cc.UpdateResource(ctx, input)
	if err != nil {
		return fmt.Errorf("error updating resource %s/%s: %w", typeName, identifier, err)
	}

	if err := obj.waitForOperation(ctx, output.ProgressEvent); err != nil {
		return fmt.Errorf("error waiting for update of %s/%s: %w", typeName, identifier, err)
	}

	return nil
}

// Delete deletes a resource by its identifier.
func (obj *Client) Delete(ctx context.Context, typeName, identifier string) error {
	input := &cloudcontrol.DeleteResourceInput{
		TypeName:   aws.String(typeName),
		Identifier: aws.String(identifier),
	}

	output, err := obj.cc.DeleteResource(ctx, input)
	if err != nil {
		return fmt.Errorf("error deleting resource %s/%s: %w", typeName, identifier, err)
	}

	if err := obj.waitForOperation(ctx, output.ProgressEvent); err != nil {
		return fmt.Errorf("error waiting for delete of %s/%s: %w", typeName, identifier, err)
	}

	return nil
}

// List returns the identifiers of all resources of the given type.
func (obj *Client) List(ctx context.Context, typeName string) ([]string, error) {
	input := &cloudcontrol.ListResourcesInput{
		TypeName: aws.String(typeName),
	}

	identifiers := []string{}

	paginator := cloudcontrol.NewListResourcesPaginator(obj.cc, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("error listing resources %s: %w", typeName, err)
		}
		for _, desc := range page.ResourceDescriptions {
			if desc.Identifier != nil {
				identifiers = append(identifiers, *desc.Identifier)
			}
		}
	}

	return identifiers, nil
}

// waitForOperation polls a Cloud Control async operation until it completes.
func (obj *Client) waitForOperation(ctx context.Context, event *cctypes.ProgressEvent) error {
	if event == nil {
		return nil
	}

	// If already complete, return immediately.
	switch event.OperationStatus {
	case cctypes.OperationStatusSuccess:
		return nil
	case cctypes.OperationStatusFailed:
		msg := ""
		if event.StatusMessage != nil {
			msg = *event.StatusMessage
		}
		return fmt.Errorf("operation failed: %s", msg)
	}

	if event.RequestToken == nil {
		return nil
	}

	ticker := time.NewTicker(WaitPollInterval)
	defer ticker.Stop()

	timeout := time.After(WaitTimeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-timeout:
			return fmt.Errorf("timed out waiting for operation")

		case <-ticker.C:
			status, err := obj.cc.GetResourceRequestStatus(ctx, &cloudcontrol.GetResourceRequestStatusInput{
				RequestToken: event.RequestToken,
			})
			if err != nil {
				return fmt.Errorf("error checking operation status: %w", err)
			}
			if status.ProgressEvent == nil {
				continue
			}
			switch status.ProgressEvent.OperationStatus {
			case cctypes.OperationStatusSuccess:
				return nil
			case cctypes.OperationStatusFailed:
				msg := ""
				if status.ProgressEvent.StatusMessage != nil {
					msg = *status.ProgressEvent.StatusMessage
				}
				return fmt.Errorf("operation failed: %s", msg)
			}
		}
	}
}

// isNotFound returns true if the error is a resource not found error.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	// Cloud Control API returns ResourceNotFoundException.
	var rnf *cctypes.ResourceNotFoundException
	if ok := errorAs(err, &rnf); ok {
		return true
	}
	return false
}

// errorAs is a helper for errors.As that works with the AWS SDK error types.
func errorAs[T error](err error, target *T) bool {
	for err != nil {
		if t, ok := err.(T); ok {
			*target = t
			return true
		}
		if u, ok := err.(interface{ Unwrap() error }); ok {
			err = u.Unwrap()
		} else {
			return false
		}
	}
	return false
}
