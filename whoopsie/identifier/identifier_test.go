package identifier

import (
	"testing"
)

// test the basics
func TestGenerate(t *testing.T) {
	id := New()
	if err := id.Generate(); err != nil {
		t.Fatalf("Generate() returned an error")
	}
	if len(id.String()) != 128 {
		t.Fatalf("Expected 128-byte id, got %d", len(id.String()))
	}
}

// test that the testing bits are enabled
func TestGenerateTestTest(test *testing.T) {
	id := New()
	id.Set("hello")
	if (id.String() != "hello") {
		test.Fatalf("Set() didn't.")
	}

	fid := Failing()
	if err := fid.Generate(); err == nil {
		test.Fatalf("Generate() on a Failing didn't fail")
	}
}
