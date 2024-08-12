// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1 "github.com/kuberhealthy/kuberhealthy/v2/pkg/apis/comcast.github.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeKuberhealthyStates implements KuberhealthyStateInterface
type FakeKuberhealthyStates struct {
	Fake *FakeComcastV1
	ns   string
}

var kuberhealthystatesResource = v1.SchemeGroupVersion.WithResource("kuberhealthystates")

var kuberhealthystatesKind = v1.SchemeGroupVersion.WithKind("KuberhealthyState")

// Get takes name of the kuberhealthyState, and returns the corresponding kuberhealthyState object, and an error if there is any.
func (c *FakeKuberhealthyStates) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.KuberhealthyState, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(kuberhealthystatesResource, c.ns, name), &v1.KuberhealthyState{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.KuberhealthyState), err
}

// List takes label and field selectors, and returns the list of KuberhealthyStates that match those selectors.
func (c *FakeKuberhealthyStates) List(ctx context.Context, opts metav1.ListOptions) (result *v1.KuberhealthyStateList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(kuberhealthystatesResource, kuberhealthystatesKind, c.ns, opts), &v1.KuberhealthyStateList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.KuberhealthyStateList{ListMeta: obj.(*v1.KuberhealthyStateList).ListMeta}
	for _, item := range obj.(*v1.KuberhealthyStateList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested kuberhealthyStates.
func (c *FakeKuberhealthyStates) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(kuberhealthystatesResource, c.ns, opts))

}

// Create takes the representation of a kuberhealthyState and creates it.  Returns the server's representation of the kuberhealthyState, and an error, if there is any.
func (c *FakeKuberhealthyStates) Create(ctx context.Context, kuberhealthyState *v1.KuberhealthyState, opts metav1.CreateOptions) (result *v1.KuberhealthyState, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(kuberhealthystatesResource, c.ns, kuberhealthyState), &v1.KuberhealthyState{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.KuberhealthyState), err
}

// Update takes the representation of a kuberhealthyState and updates it. Returns the server's representation of the kuberhealthyState, and an error, if there is any.
func (c *FakeKuberhealthyStates) Update(ctx context.Context, kuberhealthyState *v1.KuberhealthyState, opts metav1.UpdateOptions) (result *v1.KuberhealthyState, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(kuberhealthystatesResource, c.ns, kuberhealthyState), &v1.KuberhealthyState{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.KuberhealthyState), err
}

// Delete takes name of the kuberhealthyState and deletes it. Returns an error if one occurs.
func (c *FakeKuberhealthyStates) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(kuberhealthystatesResource, c.ns, name, opts), &v1.KuberhealthyState{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeKuberhealthyStates) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(kuberhealthystatesResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1.KuberhealthyStateList{})
	return err
}

// Patch applies the patch and returns the patched kuberhealthyState.
func (c *FakeKuberhealthyStates) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.KuberhealthyState, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(kuberhealthystatesResource, c.ns, name, pt, data, subresources...), &v1.KuberhealthyState{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.KuberhealthyState), err
}