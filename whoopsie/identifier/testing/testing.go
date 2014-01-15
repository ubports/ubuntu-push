package testing

import "errors"

// lets you set the value of the identifier for testing
type SettableIdentifier struct {
	value string
}

func Settable() *SettableIdentifier {
	return &SettableIdentifier{}
}

func (sid *SettableIdentifier) Set(value string) {
	sid.value = value
}

func (sid *SettableIdentifier) Generate() error {
	return nil
}

func (sid *SettableIdentifier) String() string {
	return sid.value
}


// always fails to generate
type FailingIdentifier struct {}

func Failing() *FailingIdentifier {
	return &FailingIdentifier{}
}

func (*FailingIdentifier) Generate() error {
	return errors.New("lp0 on fire")
}

func (*FailingIdentifier) String() string {
	return "<FAIL>"
}
