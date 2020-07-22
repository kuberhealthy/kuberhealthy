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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

// KuberhealthyCheckClient holds client data for talking to Kubernetes about
// the khstate custom resource
type KuberhealthyCheckClient struct {
	restClient rest.Interface
}

// Create creates a new resource for this CRD
func (c *KuberhealthyCheckClient) Create(check *KuberhealthyCheck, resource string, namespace string) (*KuberhealthyCheck, error) {
	result := KuberhealthyCheck{}
	err := c.restClient.
		Post().
		Namespace(namespace).
		Resource(resource).
		Body(check).
		Do().
		Into(&result)
	return &result, err
}

// Delete deletes a resource for this CRD
func (c *KuberhealthyCheckClient) Delete(resource string, name string, namespace string) (*KuberhealthyCheck, error) {
	result := KuberhealthyCheck{}
	err := c.restClient.
		Delete().
		Namespace(namespace).
		Resource(resource).
		Name(name).
		Do().
		Into(&result)
	return &result, err
}

// Update updates a resource for this CRD
func (c *KuberhealthyCheckClient) Update(check *KuberhealthyCheck, resource string, namespace string, name string) (*KuberhealthyCheck, error) {
	result := KuberhealthyCheck{}
	// err := c.restClient.Verb("update").Namespace(c.ns).Resource(resource).Name(name).Do().Into(&result)
	err := c.restClient.
		Put().
		Namespace(namespace).
		Resource(resource).
		Body(check).
		Name(name).
		Do().
		Into(&result)
	return &result, err
}

// Get fetches a resource of this CRD
func (c *KuberhealthyCheckClient) Get(opts metav1.GetOptions, resource string, namespace string, name string) (*KuberhealthyCheck, error) {
	result := KuberhealthyCheck{}
	err := c.restClient.
		Get().
		Namespace(namespace).
		Resource(resource).
		Name(name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(&result)
	return &result, err
}

// List lists resources for this CRD
func (c *KuberhealthyCheckClient) List(opts metav1.ListOptions, resource string, namespace string) (*KuberhealthyCheckList, error) {
	result := KuberhealthyCheckList{}
	err := c.restClient.
		Get().
		Namespace(namespace).
		Resource(resource).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(&result)
	return &result, err
}

// Watch returns a watch.Interface that watches the requested clusterTestTypes.
//func (c *KuberhealthyCheckClient) Watch(opts metav1.ListOptions, resource, namespace string) (watch.Interface, error) {
func (c *KuberhealthyCheckClient) Watch(opts metav1.ListOptions, resource string, namespace string) (watch.Interface, error) {

	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true

	return c.restClient.Get().
		Resource(resource).
		Namespace(namespace).
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch()
}
