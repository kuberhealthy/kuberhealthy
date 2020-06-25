package main

import (
	"os"
	"strings"
	"testing"
)

func TestCreateStorageConfig(t *testing.T) {
	cases := []string{"test-image:latest", "nginx:latest", "nginx:test"}
	for _, c := range cases {
		storageConfig := createStorageConfig(c)

		if len(storageConfig.Name) == 0 {
			t.Fatalf("nil container name: %s\n", storageConfig.Name)
		}

		if len(storageConfig.Namespace) == 0 {
			t.Fatalf("nil container namespace: %s\n", storageConfig.Namespace)
		}

		if storageConfig.Spec.StorageClassName != nil {
			t.Fatalf("default storage class name should be nil: %s\n", *storageConfig.Spec.StorageClassName)
		}

	}
}
func TestCreateStorageConfigFromEnv(t *testing.T) {
	storageclassName := "storagetest"
	os.Setenv("CHECK_STORAGE_PVC_STORAGE_CLASS_NAME", storageclassName)
	os.Setenv("CHECK_STORAGE_ALLOWED_CHECK_NODES", "")

	cases := []string{"test-image:latest", "nginx:latest", "nginx:test"}
	for _, c := range cases {
		storageConfig := createStorageConfig(c)
		t.Logf("storageConfig=%s\n", storageConfig)

		if len(storageConfig.Name) == 0 {
			t.Fatalf("nil container name: %s\n", storageConfig.Name)
		}

		if len(storageConfig.Namespace) == 0 {
			t.Fatalf("nil container namespace: %s\n", storageConfig.Namespace)
		}

		if storageConfig.Spec.StorageClassName == nil {
			t.Fatalf("env specified storage class name should not be nil : env is: %s\n", os.Getenv("CHECK_STORAGE_PVC_STORAGE_CLASS_NAME"))
		}
		if *storageConfig.Spec.StorageClassName != storageclassName {
			t.Fatalf("env specified storage class name should be %s: env is: %s\n", storageclassName, os.Getenv("CHECK_STORAGE_PVC_STORAGE_CLASS_NAME"))
		}

	}
}

func TestInitializeStorageConfig(t *testing.T) {
	cases := []string{"alpine:3.11", "alpine:3.11"}
	for _, c := range cases {
		initStorageConfig := initializeStorageConfig(c, c)

		if len(initStorageConfig.ObjectMeta.Name) == 0 {
			t.Fatalf("nil deployment object meta name: %s\n", initStorageConfig.ObjectMeta.Name)
		}

		if strings.Contains(initStorageConfig.ObjectMeta.Name, "-init-job") {
			t.Fatalf("Storage Config Init JOb name does not containt -init-job: %s\n", initStorageConfig.ObjectMeta.Name)
		}
		if len(initStorageConfig.ObjectMeta.Namespace) == 0 {
			t.Fatalf("nil deployment object meta namespace: %s\n", initStorageConfig.ObjectMeta.Namespace)
		}

		if initStorageConfig.Spec.Template.Spec.Containers[0].Image != c {
			t.Fatalf("expected container image to be %s but got: %s\n", c, initStorageConfig.Spec.Template.Spec.Containers[0].Image)
		}

		if len(initStorageConfig.Spec.Template.Spec.Containers[0].Image) == 0 {
			t.Fatalf("nil container image: %s\n", initStorageConfig.Spec.Template.Spec.Containers[0].Image)
		}

		if len(initStorageConfig.Spec.Template.Spec.Containers[0].ImagePullPolicy) == 0 {
			t.Fatalf("nil image pull policy: %s", initStorageConfig.Spec.Template.Spec.Containers[0].ImagePullPolicy)
		}
		/*
			reps := int32(1)
			if *deploymentConfig.Spec.Replicas < reps {
				t.Fatalf("deployment config was created with less than 1 replica: %d", deploymentConfig.Spec.Replicas)
			}

			if deploymentConfig.Spec.Selector == nil {
				t.Fatalf("deployment config was created without selctors: %v", deploymentConfig.Spec.Selector)
			}

			if len(deploymentConfig.Spec.Selector.MatchLabels) == 0 {
				t.Fatalf("deployment config was created without selctor match labels: %v", deploymentConfig.Spec.Selector)
			}
		*/
	}
}

/*
func Test_createStorageConfig(t *testing.T) {
	type args struct {
		pvcname string
	}
	tests := []struct {
		name string
		args args
		want *corev1.PersistentVolumeClaim
	}{
		// TODO: Add test cases.
		{name: "blah", args: {}, want: &corev1.PersistentVolumeClaim{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := createStorageConfig(tt.args.pvcname); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("createStorageConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}
*/
