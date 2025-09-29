package webhook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jsonpatch "github.com/evanphx/json-patch/v5"
	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	yaml "sigs.k8s.io/yaml"
)

// TestConvert upgrades a v1 check to the current API version via the conversion webhook.
func TestConvert(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping webhook conversion test in short mode")
	}
	ri := metav1.Duration{Duration: 5 * time.Minute}
	to := metav1.Duration{Duration: 2 * time.Minute}
	old := &khapi.KuberhealthyCheck{
		TypeMeta:   metav1.TypeMeta{APIVersion: "comcast.github.io/v1", Kind: "KuberhealthyCheck"},
		ObjectMeta: metav1.ObjectMeta{Name: "ssh-check", Namespace: "kuberhealthy"},
		Spec: khapi.KuberhealthyCheckSpec{
			RunInterval: &ri,
			Timeout:     &to,
			PodSpec:     khapi.CheckPodSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "ssh-check", Image: "test"}}}},
		},
	}

	oldRaw, err := json.Marshal(old)
	require.NoError(t, err)

	ar := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID:    "123",
			Object: runtimeRawExtension(oldRaw),
		},
	}
	body, err := json.Marshal(ar)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/convert", bytes.NewReader(body))
	w := httptest.NewRecorder()

	Convert(w, req)

	res := w.Result()
	defer res.Body.Close()
	require.Equal(t, 200, res.StatusCode)

	out := admissionv1.AdmissionReview{}
	err = json.NewDecoder(res.Body).Decode(&out)
	require.NoError(t, err)

	require.True(t, out.Response.Allowed)
	require.Len(t, out.Response.Warnings, 1)

	patch, err := jsonpatch.DecodePatch(out.Response.Patch)
	require.NoError(t, err)
	patched, err := patch.Apply(oldRaw)
	require.NoError(t, err)

	converted := khapi.KuberhealthyCheck{}
	err = json.Unmarshal(patched, &converted)
	require.NoError(t, err)

	require.Equal(t, "kuberhealthy.github.io/v2", converted.APIVersion)
}

// TestConvertLegacySpec converts a legacy YAML check definition to the current API version.
func TestConvertLegacySpec(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping legacy webhook conversion test in short mode")
	}
	// legacy YAML representing a comcast.github.io/v1 check
	legacy := `apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: ssh-check
  namespace: kuberhealthy
spec:
  runInterval: 5m
  timeout: 2m
  extraAnnotations:
    comcast.com/testAnnotation: test.annotation
  extraLabels:
    testLabel: testLabel
  podSpec:
    containers:
    - name: ssh-check
      image: rjacks161/ssh-check:v1.0.0
      imagePullPolicy: IfNotPresent
      env:
      - name: SSH_PRIVATE_KEY
        value: "CHANGE_ME"
      - name: SSH_USERNAME
        value: "CHANGEME"
      - name: SSH_EXCLUDE_LIST
        value: "CHANGEME1 CHANGEME2"
      resources:
        requests:
          cpu: 10m
          memory: 50Mi
`

	// convert YAML to JSON for the AdmissionReview
	legacyJSON, err := yaml.YAMLToJSON([]byte(legacy))
	require.NoError(t, err)

	ar := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID:    "456",
			Object: runtimeRawExtension(legacyJSON),
		},
	}
	body, err := json.Marshal(ar)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/convert", bytes.NewReader(body))
	w := httptest.NewRecorder()

	Convert(w, req)

	res := w.Result()
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)

	out := admissionv1.AdmissionReview{}
	err = json.NewDecoder(res.Body).Decode(&out)
	require.NoError(t, err)

	patch, err := jsonpatch.DecodePatch(out.Response.Patch)
	require.NoError(t, err)

	patched, err := patch.Apply(legacyJSON)
	require.NoError(t, err)

	converted := &khapi.KuberhealthyCheck{}
	err = json.Unmarshal(patched, converted)
	require.NoError(t, err)

	require.Equal(t, "kuberhealthy.github.io/v2", converted.APIVersion)

	// ensure the converted object marshals without error
	_, err = json.Marshal(converted)
	require.NoError(t, err)
}

// TestConvertDeploymentCheckSpec verifies that the legacy deployment check converts to v2 without dropping required pod fields.
func TestConvertDeploymentCheckSpec(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping deployment webhook conversion test in short mode")
	}

	legacy := `apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: deployment
  namespace: kuberhealthy
spec:
  runInterval: 10m
  timeout: 15m
  podSpec:
    containers:
      - name: deployment
        image: kuberhealthy/deployment-check:v1.9.1
        imagePullPolicy: IfNotPresent
        env:
          - name: CHECK_DEPLOYMENT_REPLICAS
            value: "4"
          - name: CHECK_DEPLOYMENT_ROLLING_UPDATE
            value: "true"
        resources:
          requests:
            cpu: 25m
            memory: 15Mi
          limits:
            cpu: 1
    restartPolicy: Never
    serviceAccountName: deployment-sa
    terminationGracePeriodSeconds: 60`

	legacyJSON, err := yaml.YAMLToJSON([]byte(legacy))
	require.NoError(t, err)

	review := admissionv1.AdmissionReview{Request: &admissionv1.AdmissionRequest{UID: "789", Object: runtimeRawExtension(legacyJSON)}}
	body, err := json.Marshal(review)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/convert", bytes.NewReader(body))
	w := httptest.NewRecorder()
	Convert(w, req)

	res := w.Result()
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)

	out := admissionv1.AdmissionReview{}
	err = json.NewDecoder(res.Body).Decode(&out)
	require.NoError(t, err)

	patch, err := jsonpatch.DecodePatch(out.Response.Patch)
	require.NoError(t, err)

	patched, err := patch.Apply(legacyJSON)
	require.NoError(t, err)

	converted := &khapi.KuberhealthyCheck{}
	err = json.Unmarshal(patched, converted)
	require.NoError(t, err)

	require.Equal(t, "kuberhealthy.github.io/v2", converted.APIVersion)
	require.Equal(t, "deployment", converted.Name)
	require.Equal(t, "kuberhealthy", converted.Namespace)

	require.NotNil(t, converted.Spec.RunInterval)
	require.Equal(t, time.Minute*10, converted.Spec.RunInterval.Duration)
	require.NotNil(t, converted.Spec.Timeout)
	require.Equal(t, time.Minute*15, converted.Spec.Timeout.Duration)

	spec := converted.Spec.PodSpec.Spec
	require.Equal(t, corev1.RestartPolicyNever, spec.RestartPolicy)
	require.Equal(t, "deployment-sa", spec.ServiceAccountName)

	require.NotNil(t, spec.TerminationGracePeriodSeconds)
	require.Equal(t, int64(60), *spec.TerminationGracePeriodSeconds)

	require.Len(t, spec.Containers, 1)
	container := spec.Containers[0]
	require.Equal(t, "deployment", container.Name)
	require.Equal(t, "kuberhealthy/deployment-check:v1.9.1", container.Image)
	require.Equal(t, corev1.PullIfNotPresent, container.ImagePullPolicy)

	require.Len(t, container.Env, 2)
	require.Equal(t, "CHECK_DEPLOYMENT_REPLICAS", container.Env[0].Name)
	require.Equal(t, "4", container.Env[0].Value)
	require.Equal(t, "CHECK_DEPLOYMENT_ROLLING_UPDATE", container.Env[1].Name)
	require.Equal(t, "true", container.Env[1].Value)

	cpuRequest := container.Resources.Requests.Cpu()
	require.NotNil(t, cpuRequest)
	require.True(t, cpuRequest.Equal(resource.MustParse("25m")))

	memoryRequest := container.Resources.Requests.Memory()
	require.NotNil(t, memoryRequest)
	require.True(t, memoryRequest.Equal(resource.MustParse("15Mi")))

	cpuLimit := container.Resources.Limits.Cpu()
	require.NotNil(t, cpuLimit)
	require.True(t, cpuLimit.Equal(resource.MustParse("1")))
}

// runtimeRawExtension wraps a byte slice inside a RawExtension for AdmissionReview payloads.
func runtimeRawExtension(b []byte) runtime.RawExtension {
	return runtime.RawExtension{Raw: b}
}
