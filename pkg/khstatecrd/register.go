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

package khstatecrd

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
)

var SchemeGroupVersion schema.GroupVersion

// ConfigureScheme configures the runtime scheme for use with CRD creation
func ConfigureScheme(GroupName string, GroupVersion string) {
	SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: GroupVersion}
	var (
		SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
		AddToScheme   = SchemeBuilder.AddToScheme
	)
	AddToScheme(scheme.Scheme)
}

// knownTypesMu works around a potential race with a map inside the kubernetes
// api machinery which crashes when addKnownTypes and AddToGroupVersion are
// both executing at the same time.
var knownTypesMu sync.Mutex

func addKnownTypes(scheme *runtime.Scheme) error {
	knownTypesMu.Lock()
	defer knownTypesMu.Unlock()

	scheme.AddKnownTypes(SchemeGroupVersion,
		&KuberhealthyState{},
		&KuberhealthyStateList{},
	)

	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
