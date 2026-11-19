package shape_test

import (
	"plainsolid/tools/refgraph/testdata/fixture/shape"
	"testing"
)

func TestFancyOutside(t *testing.T) {
	if shape.Fancy(shape.New()) != 6 {
		t.Fatal("fancy")
	}
}
