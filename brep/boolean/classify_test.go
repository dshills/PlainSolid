package boolean

import (
	"errors"
	"testing"

	"plainsolid/brep"
)

// TestClassify covers K1 to K3 on box pairs: the number of pieces of
// each operand in each class.
func TestClassify(t *testing.T) {
	cube := box(t, 0, 0, 0, 2, 2, 2)
	for _, c := range []struct {
		name string
		b    brep.Solid
		want [2][4]int // by operand: outside, inside, shared same, shared opposite
	}{
		{"C3 inside without contact", box(t, 0.5, 0.5, 0.5, 1.5, 1.5, 1.5), [2][4]int{{6, 0, 0, 0}, {0, 6, 0, 0}}},
		{"C4 whole face shared", box(t, 0, 0, 2, 2, 2, 3), [2][4]int{{5, 0, 0, 1}, {5, 0, 0, 1}}},
		{"C5 part of a face shared", box(t, 1, 0, 2, 3, 2, 3), [2][4]int{{6, 0, 0, 1}, {6, 0, 0, 1}}},
		{"C6 three faces flush", box(t, 0, 0, 0, 1, 1, 1), [2][4]int{{6, 0, 3, 0}, {0, 3, 3, 0}}},
		{"corner overlap", box(t, 1, 1, 1, 3, 3, 3), [2][4]int{{6, 3, 0, 0}, {6, 3, 0, 0}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			a, err := whole(t, cube, c.b, nil)
			if err != nil {
				t.Fatal(err)
			}
			pieces, err := a.split()
			if err != nil {
				t.Fatal(err)
			}
			if err := a.classify(pieces); err != nil {
				t.Fatal(err)
			}
			var got [2][4]int
			for _, p := range pieces {
				got[p.op][p.class]++
				if (p.mate >= 0) != (p.class == sharedSame || p.class == sharedOpposite) {
					t.Fatalf("piece %+v: the shared face does not go with the class", p)
				}
			}
			if got != c.want {
				t.Fatalf("classes by operand: got %v, want %v", got, c.want)
			}
		})
	}
	t.Run("K3", func(t *testing.T) {
		// Two faces of A that meet along an edge B does not touch must
		// not differ in class.
		a, err := whole(t, cube, box(t, 0.5, 0.5, 0.5, 1.5, 1.5, 1.5), nil)
		if err != nil {
			t.Fatal(err)
		}
		pieces, err := a.split()
		if err != nil {
			t.Fatal(err)
		}
		pieces[0].class = inside
		if err := a.sameAcross(pieces); !errors.Is(err, ErrInconsistent) {
			t.Fatalf("got %v, want ErrInconsistent", err)
		}
	})
}
