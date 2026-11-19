package consumer

import (
	"plainsolid/tools/refgraph/testdata/fixture/shape"
	"testing"
)

func TestFancy(t *testing.T) { _ = shape.Fancy(Make()) }
