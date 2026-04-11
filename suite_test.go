package pgxprom

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPgxprom(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "pgxprom Suite")
}
