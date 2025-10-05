package jobwebhook

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TestConvertJob upgrades a legacy job to a v2 check via the conversion webhook.
func TestConvertJob(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping job conversion test in short mode")
	}
	ri := metav1.Duration{Duration: 5 * time.Minute}
	to := metav1.Duration{Duration: 2 * time.Minute}
	job := khJob{
		TypeMeta:   metav1.TypeMeta{APIVersion: "comcast.github.io/v1", Kind: "KuberhealthyJob"},
		ObjectMeta: metav1.ObjectMeta{Name: "legacy-job", Namespace: "kuberhealthy"},
		Spec: khJobSpec{
			RunOnce:          true,
			RunInterval:      &ri,
			Timeout:          &to,
			ExtraAnnotations: map[string]string{"a": "b"},
			ExtraLabels:      map[string]string{"c": "d"},
			PodSpec:          corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "job", Image: "test"}}}},
		},
	}
	jobRaw, err := json.Marshal(job)
	require.NoError(t, err)

	review := apiextv1.ConversionReview{Request: &apiextv1.ConversionRequest{UID: "123", Objects: []runtime.RawExtension{{Raw: jobRaw}}}}
	body, err := json.Marshal(review)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/khjobconvert", bytes.NewReader(body))
	w := httptest.NewRecorder()
	Convert(w, req)

	res := w.Result()
	defer res.Body.Close()
	require.Equal(t, 200, res.StatusCode)

	out := apiextv1.ConversionReview{}
	err = json.NewDecoder(res.Body).Decode(&out)
	require.NoError(t, err)
	require.NotNil(t, out.Response)
	require.Equal(t, "123", string(out.Response.UID))
	require.Len(t, out.Response.ConvertedObjects, 1)

	converted := khapi.HealthCheck{}
	err = json.Unmarshal(out.Response.ConvertedObjects[0].Raw, &converted)
	require.NoError(t, err)
	require.Equal(t, "kuberhealthy.github.io/v2", converted.APIVersion)
	require.True(t, converted.Spec.SingleRun)
	require.Equal(t, job.Spec.Timeout, converted.Spec.Timeout)
	require.Equal(t, "b", converted.Spec.ExtraAnnotations["a"])
	require.Equal(t, "d", converted.Spec.ExtraLabels["c"])
	require.Equal(t, "test", converted.Spec.PodSpec.Spec.Containers[0].Image)
}
