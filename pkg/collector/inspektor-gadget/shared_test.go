package inspektor_gadget

import (
	"testing"

	"github.com/Azure/aks-periscope/pkg/test"
)

// TestMain coordinates the execution of all tests in the package. This is required because they all share
// common initialization and cleanup code.
func TestMain(m *testing.M) {
	test.RunPackageTests(m)
}
