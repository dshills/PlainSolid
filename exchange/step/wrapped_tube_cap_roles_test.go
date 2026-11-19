package step

import (
	"bytes"
	"os"
	"testing"
)

func TestWrappedTubeCapRoles(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/wrapped-tube.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedTube(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteWrappedTube(&b, m); e != nil {
		t.Fatal(e)
	}
	for _, data := range [][]byte{raw, b.Bytes()} {
		f, e := Parse(bytes.NewReader(data), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		changed := 0
		for _, id := range f.Order {
			face, e := simple(f, id, "ADVANCED_FACE", 4)
			if e != nil {
				continue
			}
			sid, _ := ref(face.Args[2])
			if _, e = simple(f, sid, "PLANE", 2); e != nil {
				continue
			}
			bounds, e := listRefs(face.Args[1], 2)
			if e != nil || len(bounds) != 2 {
				t.Fatal("cap bounds")
			}
			for _, bid := range bounds {
				bound := f.Entities[bid].Records[0]
				lid, _ := ref(bound.Args[1])
				loop, _ := simple(f, lid, "EDGE_LOOP", 2)
				uses, _ := listRefs(loop.Args[1], 1)
				use, _ := simple(f, uses[0], "ORIENTED_EDGE", 5)
				sense, _ := boolean(use.Args[4])
				if sense {
					continue
				}
				if bound.Type != "FACE_BOUND" {
					t.Fatal("hole emitted as outer bound")
				}
				bound.Type = "FACE_OUTER_BOUND"
				ovalSet(f, bid, bound)
				changed++
			}
		}
		if changed != 2 {
			t.Fatal(changed)
		}
		if _, e = ReadWrappedTube(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, WrappedSurfaceOptions{}); e == nil {
			t.Fatal("hole outer role accepted")
		}
	}
}
