/* Copyright 2018 Comcast Cable Communications Management, LLC
   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at
       http://www.apache.org/licenses/LICENSE-2.0
   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/
package khstatecrd // import "github.com/Comcast/kuberhealthy/pkg/khstatecrd"

import (
	"os"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var namespace = os.Getenv("POD_NAMESPACE")

const resource = "khstates"
const group = "comcast.github.io"
const version = "v1"

// Client creates a rest client to use for interacting with CRDs
func Client(GroupName string, GroupVersion string, kubeConfig string) (*KuberhealthyStateClient, error) {

	var c *rest.Config
	var err error

	c, err = rest.InClusterConfig()
	if err != nil {
		c, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
	}

	if err != nil {
		return &KuberhealthyStateClient{}, err
	}

	ConfigureScheme(GroupName, GroupVersion)

	config := *c
	config.ContentConfig.GroupVersion = &schema.GroupVersion{Group: GroupName, Version: GroupVersion}
	config.APIPath = "/apis"
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}
	config.UserAgent = rest.DefaultKubernetesUserAgent()

	client, err := rest.RESTClientFor(&config)
	return &KuberhealthyStateClient{restClient: client, ns: namespace}, err
}
