//go:build integration

package apitest

import "testing"

// TestGeneratedMaterialTypeCRUD exposes the generated CRUD scenario to go test.
// Codegen currently writes the scenario to a *.gen.go file, which Go does not
// discover as a test file on its own.
func TestGeneratedMaterialTypeCRUD(t *testing.T) {
	TestMaterialTypeCRUD(t)
}
