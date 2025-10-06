package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGetMyNamespaceTrimsWhitespace verifies that namespace values are trimmed before being returned.
func TestGetMyNamespaceTrimsWhitespace(t *testing.T) {

	// preserve the original namespace path so the test can restore it
	originalPath := serviceAccountNamespacePath
	t.Cleanup(func() {
		serviceAccountNamespacePath = originalPath
	})

	// create a temporary namespace file containing trailing whitespace
	tempDir := t.TempDir()
	namespaceFile := filepath.Join(tempDir, "namespace")
	err := os.WriteFile(namespaceFile, []byte("example-namespace\n"), 0o644)
	if err != nil {
		t.Fatalf("failed to write namespace file: %v", err)
	}

	// point the function under test at the temporary namespace file
	serviceAccountNamespacePath = namespaceFile

	// ensure the returned namespace is trimmed as expected
	namespace := GetMyNamespace("default")
	if namespace != "example-namespace" {
		t.Fatalf("unexpected namespace value: %q", namespace)
	}
}

// TestGetMyNamespaceFallsBackToDefault verifies that the default namespace is returned when the file is empty.
func TestGetMyNamespaceFallsBackToDefault(t *testing.T) {

	// preserve the original namespace file path to avoid leaking state between tests
	originalPath := serviceAccountNamespacePath
	t.Cleanup(func() {
		serviceAccountNamespacePath = originalPath
	})

	// create an empty namespace file to simulate an unreadable value
	tempDir := t.TempDir()
	namespaceFile := filepath.Join(tempDir, "namespace")
	err := os.WriteFile(namespaceFile, []byte(" \t\n"), 0o644)
	if err != nil {
		t.Fatalf("failed to write namespace file: %v", err)
	}

	// point GetMyNamespace at the empty namespace file and verify the default is returned
	serviceAccountNamespacePath = namespaceFile
	namespace := GetMyNamespace("fallback")
	if namespace != "fallback" {
		t.Fatalf("expected fallback namespace, got %q", namespace)
	}
}
