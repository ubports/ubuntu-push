package testing

import (
	. "launchpad.net/gocheck"
	"testing"
	identifier ".."
)

// hook up gocheck
func Test(t *testing.T) { TestingT(t) }

type IdentifierSuite struct{}
var _ = Suite(&IdentifierSuite{})

func (s *IdentifierSuite) TestTesting(c *C) {
	id := Settable()
	id.Set("hello")
	c.Check(id.String(), Equals, "hello")

	fid := Failing()
	c.Check(fid.Generate(), Not(Equals), nil)
}

//tests the interfaces of the different classes
func (s *IdentifierSuite) TestIdentifierInterface(c *C) {
	_ = []identifier.Id{Failing(), Settable()}
}
