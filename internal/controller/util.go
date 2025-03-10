package controller

func containsString(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}

func removeString(slice []string, str string) []string {
	var newSlice []string
	for _, v := range slice {
		if v != str {
			newSlice = append(newSlice, v)
		}
	}
	return newSlice
}
