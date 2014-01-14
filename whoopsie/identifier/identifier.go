package identifier

/*
#cgo pkg-config: libwhoopsie
#include <glib.h>
#include <libwhoopsie/identifier.h>
*/
import "C"
import "unsafe"
import "errors"


type Id interface {
	Generate() error
	String() string
}

type Identifier struct {
	value string
}

func New() Identifier {
	return Identifier{""}
}

func (self Identifier) String() string {
	return self.value
}

func (self *Identifier) Generate() error {
	var gerr *C.GError
	var cs *C.char
	defer C.g_free((C.gpointer)(unsafe.Pointer(cs)))
	C.whoopsie_identifier_generate(&cs, &gerr)

	if (gerr != nil) {
		return errors.New(C.GoString((*C.char)(gerr.message)));
	} else {
		self.value = C.GoString(cs)
		return nil
	}

}
