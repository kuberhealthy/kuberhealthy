package khcheckcrd

// CreateTestCheck creates a test placeholder check for testing and debugging
func CreateTestCheck(kubeConfigFile string, checkName string) error {

	// create a CRD client
	client, err := Client(group, version, kubeConfigFile)
	if err != nil {
		return err
	}

	// create a new skeleton check
	check := NewKuberhealthyCheck(checkName, NewCheckConfig())

	// create the check against the kubernetes API
	_, err = client.Create(&check, resource)
	return err
}
