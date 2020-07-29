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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// KuberhealthyStateList is a struct from the module k8s.io/apimachinery/pkg/apis/meta/v1"
type KuberhealthyStateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KuberhealthyState `json:"items"`
}

// DeepCopyInto copies all properties of this object into another object of the
// same type that is provided as a pointer.
func (h *KuberhealthyStateList) DeepCopyInto(out *KuberhealthyStateList) {
	out.TypeMeta = h.TypeMeta
	out.ListMeta = h.ListMeta
	if h.Items != nil {
		out.Items = make([]KuberhealthyState, len(h.Items))
		for i := range h.Items {
			h.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

// DeepCopyObject returns a generically typed copy of an object
func (h *KuberhealthyStateList) DeepCopyObject() runtime.Object {
	out := KuberhealthyStateList{}
	h.DeepCopyInto(&out)

	return &out
}
