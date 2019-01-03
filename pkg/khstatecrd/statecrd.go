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
package khstatecrd

import (
	"encoding/json"

	"github.com/Comcast/kuberhealthy/pkg/health"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type KuberhealthyState struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              health.CheckDetails `json:"spec"`
}

// String satisfies the stringer interface for cleaner output when printing
func (h KuberhealthyState) String() string {
	b, err := json.MarshalIndent(&h, "", "\t")
	if err != nil {
		logrus.Errorln("Failed to marshal KuberhealthyState in a nice format:", err)
	}
	return string(b)
}

// DeepCopyInto copies all properties of this object into another object of the
// same type that is provided as a pointer.
func (h KuberhealthyState) DeepCopyInto(out *KuberhealthyState) {
	out.TypeMeta = h.TypeMeta
	out.ObjectMeta = h.ObjectMeta
	out.Spec = h.Spec
}

// DeepCopyObject returns a generically typed copy of an object
func (h KuberhealthyState) DeepCopyObject() runtime.Object {
	out := KuberhealthyState{}
	h.DeepCopyInto(&out)
	return &out
}

// NewKuberhealthyState creates a KuberhealthyState struct which represents
// the data inside a KuberHealthyCheck resource
func NewKuberhealthyState(name string, spec health.CheckDetails) KuberhealthyState {
	state := KuberhealthyState{}
	state.SetName(name)
	state.Spec = spec
	return state
}
