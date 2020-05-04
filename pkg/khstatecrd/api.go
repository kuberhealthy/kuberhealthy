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

package khstatecrd // import "github.com/Comcast/kuberhealthy/v2/pkg/khstatecrd"

import (
	"log"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const defaultNamespace = "kuberhealthy"

const resource = "khstates"
const group = "comcast.github.io"
const version = "v1"

// Client creates a rest client to use for interacting with CRDs
func Client(GroupName string, GroupVersion string, kubeConfig string, namespace string) (*KuberhealthyStateClient, error) {

	var c *rest.Config
	var err error

	// log.Println("Loading in-cluster kubernetes config...")
	c, err = rest.InClusterConfig()
	if err != nil {
		log.Println("Loading config from flags...")
		c, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
	}

	if err != nil {
		return &KuberhealthyStateClient{}, err
	}

	// log.Println("Configuring scheme")
	err = ConfigureScheme(GroupName, GroupVersion)
	if err != nil {
		return &KuberhealthyStateClient{}, err
	}

	config := *c
	config.ContentConfig.GroupVersion = &schema.GroupVersion{Group: GroupName, Version: GroupVersion}
	config.APIPath = "/apis"
	// config.NegotiatedSerializer = serializer.NegotiatedSerializerWrapper(){CodecFactory: scheme.Codecs}
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}
	config.UserAgent = rest.DefaultKubernetesUserAgent()

	// log.Println("creating khstate rest client")
	client, err := rest.RESTClientFor(&config)
	return &KuberhealthyStateClient{restClient: client}, err
}
