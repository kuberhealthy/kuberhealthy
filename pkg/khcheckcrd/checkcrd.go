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
	"encoding/json"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// KuberhealthyCheck represents the data in the CRD for configuring an
// external checker for Kuberhealthy
type KuberhealthyCheck struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              CheckConfig `json:"spec"`
}

// String satisfies the stringer interface for cleaner output when printing
func (h KuberhealthyCheck) String() string {
	b, err := json.MarshalIndent(&h, "", "\t")
	if err != nil {
		logrus.Errorln("Failed to marshal KuberhealthyCheck in a nice format:", err)
	}
	return string(b)
}

// DeepCopyInto copies all properties of this object into another object of the
// same type that is provided as a pointer.
func (h KuberhealthyCheck) DeepCopyInto(out *KuberhealthyCheck) {
	out.TypeMeta = h.TypeMeta
	out.ObjectMeta = h.ObjectMeta
	out.Spec = h.Spec
}

// DeepCopyObject returns a generically typed copy of an object
func (h KuberhealthyCheck) DeepCopyObject() runtime.Object {
	out := KuberhealthyCheck{}
	h.DeepCopyInto(&out)
	return &out
}

// NewKuberhealthyCheck creates a KuberhealthyCheck struct which represents
// the data inside a KuberhealthyCheck resource
func NewKuberhealthyCheck(name string, namespace string, spec CheckConfig) KuberhealthyCheck {
	check := KuberhealthyCheck{}
	check.Name = name
	check.ObjectMeta.Name = name
	check.Kind = "KuberhealthyCheck"
	check.Spec = spec
	check.Namespace = namespace
	check.ObjectMeta.Namespace = namespace
	return check
}
