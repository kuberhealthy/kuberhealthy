package main

import (
	"context"
	"os"
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_findPodsNotRunning(t *testing.T) {
	objects := getTestPods()
	os.Setenv("SKIP_DURATION", "10m")

	type fields struct {
		objects   []runtime.Object
		namespace string
	}
	tests := []struct {
		name    string
		fields  fields
		want    []string
		wantErr bool
	}{
		{name: "single_namespace", fields: fields{
			namespace: "foo",
		}, want: []string{"pod: foo-pod in namespace: foo is in pod status phase Pending "}, wantErr: false},
		{name: "multi_namespace", fields: fields{
			namespace: "",
		}, want: []string{"pod: bar-pod in namespace: bar is in pod status phase Pending ", "pod: foo-pod in namespace: foo is in pod status phase Pending "}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			os.Setenv("TARGET_NAMESPACE", tt.fields.namespace)

			client := fake.NewSimpleClientset(objects...)
			o := Options{
				client: client,
			}
			got, err := o.findPodsNotRunning(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("findErrors() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findErrors() got = %v, want %v", got, tt.want)
			}
		})
	}

}

func getTestPods() []runtime.Object {

	return []runtime.Object{
		&v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
		},
		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo-pod",
				Namespace: "foo",
			},
			Status: v1.PodStatus{
				Phase: v1.PodPending,
			},
		},
		&v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bar",
			},
		},
		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bar-pod",
				Namespace: "bar",
			},
			Status: v1.PodStatus{
				Phase: v1.PodPending,
			},
		},
	}
}
