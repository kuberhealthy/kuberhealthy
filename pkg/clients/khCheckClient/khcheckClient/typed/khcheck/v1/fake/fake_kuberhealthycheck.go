// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1 "github.com/kuberhealthy/kuberhealthy/v2/pkg/apis/khcheck/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeKuberhealthyChecks implements KuberhealthyCheckInterface
type FakeKuberhealthyChecks struct {
	Fake *FakeComcastV1
	ns   string
}

var kuberhealthychecksResource = v1.SchemeGroupVersion.WithResource("kuberhealthychecks")

var kuberhealthychecksKind = v1.SchemeGroupVersion.WithKind("KuberhealthyCheck")

// Get takes name of the kuberhealthyCheck, and returns the corresponding kuberhealthyCheck object, and an error if there is any.
func (c *FakeKuberhealthyChecks) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.KuberhealthyCheck, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(kuberhealthychecksResource, c.ns, name), &v1.KuberhealthyCheck{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.KuberhealthyCheck), err
}

// List takes label and field selectors, and returns the list of KuberhealthyChecks that match those selectors.
func (c *FakeKuberhealthyChecks) List(ctx context.Context, opts metav1.ListOptions) (result *v1.KuberhealthyCheckList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(kuberhealthychecksResource, kuberhealthychecksKind, c.ns, opts), &v1.KuberhealthyCheckList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.KuberhealthyCheckList{ListMeta: obj.(*v1.KuberhealthyCheckList).ListMeta}
	for _, item := range obj.(*v1.KuberhealthyCheckList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested kuberhealthyChecks.
func (c *FakeKuberhealthyChecks) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(kuberhealthychecksResource, c.ns, opts))

}

// Create takes the representation of a kuberhealthyCheck and creates it.  Returns the server's representation of the kuberhealthyCheck, and an error, if there is any.
func (c *FakeKuberhealthyChecks) Create(ctx context.Context, kuberhealthyCheck *v1.KuberhealthyCheck, opts metav1.CreateOptions) (result *v1.KuberhealthyCheck, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(kuberhealthychecksResource, c.ns, kuberhealthyCheck), &v1.KuberhealthyCheck{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.KuberhealthyCheck), err
}

// Update takes the representation of a kuberhealthyCheck and updates it. Returns the server's representation of the kuberhealthyCheck, and an error, if there is any.
func (c *FakeKuberhealthyChecks) Update(ctx context.Context, kuberhealthyCheck *v1.KuberhealthyCheck, opts metav1.UpdateOptions) (result *v1.KuberhealthyCheck, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(kuberhealthychecksResource, c.ns, kuberhealthyCheck), &v1.KuberhealthyCheck{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.KuberhealthyCheck), err
}

// Delete takes name of the kuberhealthyCheck and deletes it. Returns an error if one occurs.
func (c *FakeKuberhealthyChecks) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(kuberhealthychecksResource, c.ns, name, opts), &v1.KuberhealthyCheck{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeKuberhealthyChecks) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(kuberhealthychecksResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1.KuberhealthyCheckList{})
	return err
}

// Patch applies the patch and returns the patched kuberhealthyCheck.
func (c *FakeKuberhealthyChecks) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.KuberhealthyCheck, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(kuberhealthychecksResource, c.ns, name, pt, data, subresources...), &v1.KuberhealthyCheck{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.KuberhealthyCheck), err
}
