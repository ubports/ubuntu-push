// +build testing

package identifier

import "errors"

// lets you set the value of the identifier for testing
func (self *Identifier) Set(value string) {
	self.value = value
}


type FailingIdentifier bool

func Failing() FailingIdentifier {
	return FailingIdentifier(false)
}

func (*FailingIdentifier) Generate() error {
	return errors.New("lp0 on fire")
}

func (FailingIdentifier) String() string {
	return "<FAIL>"
}
