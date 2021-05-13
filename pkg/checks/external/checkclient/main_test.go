package checkclient

import (
	"os"
	"testing"
	"time"

	"github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external"
)

// TestGetKuberhealthyURL ensures that KH_REPORTING_URL env var can be fetched
func TestGetKuberhealthyURL(t *testing.T) {

	var testCases = []struct {
		input string
		err   string
	}{
		{"http://kuberhealthy.kuberhealthy.svc.cluster.local/externalCheckStatus", ""},
		{"http://anotherurl.com/externalCheckStatus", ""},
		{"", "fetched KH_REPORTING_URL environment variable but it was blank"},
	}

	for _, tc := range testCases {

		os.Setenv(external.KHReportingURL, tc.input)
		result, err := getKuberhealthyURL()
		if err != nil {
			if err.Error() != tc.err {
				t.Fatalf("getKuberhealthyURL err is `%s` but expected err `%s`", err.Error(), tc.err)
			}
			t.Logf("getKuberhealthyURL err resulted in `%s` correctly", tc.err)
			continue
		}
		if result != tc.input {
			t.Fatalf("getKuberhealthyURL resulted in `%s` but expected result `%s`", result, tc.input)
		}

		t.Logf("getKuberhealthyURL resulted in `%s` correctly", result)
	}
}

// TestGetKuberhealthyRunUUID ensures that KH_RUN_UUID env var can be fetched
func TestGetKuberhealthyRunUUID(t *testing.T) {

	var testCases = []struct {
		input string
		err   string
	}{
		{"some-random-uuid", ""},
		{"another-random-uuid, not allowed", ""},
		{"", "fetched KH_RUN_UUID environment variable but it was blank"},
	}

	for _, tc := range testCases {

		os.Setenv(external.KHRunUUID, tc.input)
		result, err := getKuberhealthyRunUUID()
		if err != nil {
			if err.Error() != tc.err {
				t.Fatalf("getKuberhealthyRunUUID err is `%s` but expected err `%s`", err.Error(), tc.err)
			}
			t.Logf("getKuberhealthyRunUUID err resulted in `%s` correctly", tc.err)
			continue
		}
		if result != tc.input {
			t.Fatalf("getKuberhealthyRunUUID resulted in `%s` but expected result `%s`", result, tc.input)
		}

		t.Logf("getKuberhealthyRunUUID resulted in `%s` correctly", result)
	}
}

// TestGetDeadline ensures that KH_CHECK_RUN_DEADLINE env var can be fetched and parsed
func TestGetDeadline(t *testing.T) {

	var testCases = []struct {
		input     string
		inputTime time.Time
		err       string
	}{
		{"1618336810", time.Unix(int64(1618336810), 0), ""},
		{"1618346824", time.Unix(int64(1618346824), 0), ""},
		{"bad-input", time.Time{}, "unable to parse KH_CHECK_RUN_DEADLINE: strconv.Atoi: parsing \"bad-input\": invalid syntax"},
		{"", time.Time{}, "fetched KH_CHECK_RUN_DEADLINE environment variable but it was blank"},
	}

	for _, tc := range testCases {

		os.Setenv(external.KHDeadline, tc.input)
		result, err := GetDeadline()
		if err != nil {
			if err.Error() != tc.err {
				t.Fatalf("GetDeadline err is `%s` but expected err `%s`", err.Error(), tc.err)
			}
			t.Logf("GetDeadline err resulted in `%s` correctly", tc.err)
			continue
		}
		if result != tc.inputTime {
			t.Fatalf("GetDeadline resulted in `%s` but expected result `%s`", result, tc.input)
		}

		t.Logf("GetDeadline resulted in `%s` correctly", result)
	}
}

//TODO: TestSendReport
