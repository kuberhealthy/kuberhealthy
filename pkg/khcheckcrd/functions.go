// Copyright 2018 Comcast Cable Communications Management, LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package khcheckcrd

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

// KuberhealthyStateClient holds client data for talking to Kubernetes about
// the khstate custom resource
type KuberhealthyStateClient struct {
	restClient rest.Interface
	ns         string
}

// Create creates a new resource for this CRD
func (c *KuberhealthyStateClient) Create(state *KuberhealthyState, resource string) (*KuberhealthyState, error) {
	result := KuberhealthyState{}
	err := c.restClient.
		Post().
		Namespace(c.ns).
		Resource(resource).
		Body(state).
		Do().
		Into(&result)
	return &result, err
}

// Delete deletes a resource for this CRD
func (c *KuberhealthyStateClient) Delete(state *KuberhealthyState, resource string, name string) (*KuberhealthyState, error) {
	result := KuberhealthyState{}
	err := c.restClient.
		Delete().
		Namespace(c.ns).
		Resource(resource).
		Name(name).
		Do().
		Into(&result)
	return &result, err
}

// Update updates a resource for this CRD
func (c *KuberhealthyStateClient) Update(state *KuberhealthyState, resource string, name string) (*KuberhealthyState, error) {
	result := KuberhealthyState{}
	// err := c.restClient.Verb("update").Namespace(c.ns).Resource(resource).Name(name).Do().Into(&result)
	err := c.restClient.
		Put().
		Namespace(c.ns).
		Resource(resource).
		Body(state).
		Name(name).
		Do().
		Into(&result)
	return &result, err
}

// Get fetches a resource of this CRD
func (c *KuberhealthyStateClient) Get(opts metav1.GetOptions, resource string, name string) (*KuberhealthyState, error) {
	result := KuberhealthyState{}
	err := c.restClient.
		Get().
		Namespace(c.ns).
		Resource(resource).
		Name(name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(&result)
	return &result, err
}

// List lists resources for this CRD
func (c *KuberhealthyStateClient) List(opts metav1.ListOptions, resource string) (*KuberhealthyStateList, error) {
	result := KuberhealthyStateList{}
	err := c.restClient.
		Get().
		Namespace(c.ns).
		Resource(resource).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(&result)
	return &result, err
}
