package framework

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
)

// Byf is used to provider better output for a test using a formatted string.
func Byf(format string, a ...interface{}) {
	By(fmt.Sprintf(format, a...))
}
