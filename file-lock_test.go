package fsutil

import (
	"testing"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }
type FileLockSuite struct {}
var _ = Suite(&FileLockSuite{})

func (a *FileLockSuite) TestX(c *C) {
	c.Check(true, Equals, true)
}
