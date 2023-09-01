package naming

import (
	"fmt"
	"strings"
)

var rancherCAPISuffix = "-capi"

// Name is a wrapper around CAPI/Rancher cluster names to simplify convertation between the two.
type Name string

// ToRancherName converts a CAPI cluster name to Rancher cluster name.
func (n Name) ToRancherName() string {
	return fmt.Sprintf("%s%s", n.ToCapiName(), rancherCAPISuffix)
}

// ToCapiName converts a Rancher cluster name to CAPI cluster name.
func (n Name) ToCapiName() string {
	return strings.TrimSuffix(string(n), rancherCAPISuffix)
}
