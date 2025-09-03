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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	yaml "sigs.k8s.io/yaml"
)

func TestConvert(t *testing.T) {
	ri := metav1.Duration{Duration: 5 * time.Minute}
	to := metav1.Duration{Duration: 2 * time.Minute}
	old := &khapi.KuberhealthyCheck{
		TypeMeta:   metav1.TypeMeta{APIVersion: "comcast.github.io/v1", Kind: "KuberhealthyCheck"},
		ObjectMeta: metav1.ObjectMeta{Name: "ssh-check", Namespace: "kuberhealthy"},
		Spec: khapi.KuberhealthyCheckSpec{
			RunInterval: &ri,
			Timeout:     &to,
			PodSpec:     corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "ssh-check", Image: "test"}}}},
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

func TestConvertLegacySpec(t *testing.T) {
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

func TestConvertRemovesPodSpecCreationTimestamp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	legacy := `apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: creation-time-test
  namespace: kuberhealthy
spec:
  podSpec:
    metadata:
      creationTimestamp: null
    containers:
    - name: ct
      image: test
`
	legacyJSON, err := yaml.YAMLToJSON([]byte(legacy))
	require.NoError(t, err)

	ar := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID:    "789",
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

	obj := map[string]any{}
	err = json.Unmarshal(patched, &obj)
	require.NoError(t, err)

	spec, ok := obj["spec"].(map[string]any)
	require.True(t, ok)
	podSpec, ok := spec["podSpec"].(map[string]any)
	require.True(t, ok)
	_, ok = podSpec["metadata"]
	require.False(t, ok)
}

func runtimeRawExtension(b []byte) runtime.RawExtension {
	return runtime.RawExtension{Raw: b}
}
