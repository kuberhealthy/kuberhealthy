package health

import "testing"

// TestNewReportDefaultsOK checks that a new report without errors is marked healthy.
func TestNewReportDefaultsOK(t *testing.T) {
	t.Parallel()
	r := NewReport(nil)
	if !r.OK {
		t.Errorf("expected OK true when errors slice is nil")
	}

	r = NewReport([]string{})
	if !r.OK {
		t.Errorf("expected OK true when errors slice is empty")
	}
}

// TestNewReportWithErrors confirms that providing errors results in a non-OK report.
func TestNewReportWithErrors(t *testing.T) {
	t.Parallel()
	errs := []string{"err1"}
	r := NewReport(errs)
	if r.OK {
		t.Errorf("expected OK false when errors are present")
	}
	if len(r.Errors) != len(errs) || r.Errors[0] != errs[0] {
		t.Errorf("unexpected errors: %v", r.Errors)
	}
}
