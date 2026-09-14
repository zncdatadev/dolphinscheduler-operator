package controller

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zncdatadev/operator-go/pkg/testutil"
)

// The role/role-group `config` block must carry no `+kubebuilder:default`: structural
// defaulting would stamp the default into every group that declares a config block for any
// reason, making it indistinguishable from an explicit value and unable to lose the merge
// against role-level settings (operator-go #573).
var _ = Describe("CRD schema", func() {
	It("declares no CRD default inside a role or role group config block", func() {
		Expect(filepath.Join("..", "..", "config", "crd", "bases", "*.yaml")).
			To(testutil.HaveNoInheritedConfigDefaults())
	})
})
