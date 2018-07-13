package podRestarts

import "testing"

func TestPodRestartChecker(t *testing.T) {
	c := New("kube-system")
	err := c.doChecks()
	if err != nil {
		t.Fatal(err)
	}
	up, errors := c.CurrentStatus()
	t.Log("up:", up)
	t.Log("errors:", errors)
	err = c.Shutdown()
	if err != nil {
		t.Fatal(err)
	}
}
