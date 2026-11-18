package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func separatedChainJSONFixture(t *testing.T, negative, reverse bool) (*WrappedAlgebraicSeparatedChain, []byte) {
	t.Helper()
	ctx := context.Background()
	ab, bc, h := wrappedSeparatedChainFixture(t, negative)
	if reverse {
		cb0, _ := bc.pair.Owner(1)
		cb1, _ := bc.pair.Owner(0)
		cb, e := NewWrappedAlgebraicOpenShell(ctx, bc.sources[1], bc.sources[0], [2]int{cb0.Loop, cb0.Edge}, [2]int{cb1.Loop, cb1.Edge}, bc.pair.ReversedParameter(), edgecheck.Options{})
		if e != nil {
			t.Fatal(e)
		}
		ba0, _ := ab.pair.Owner(1)
		ba1, _ := ab.pair.Owner(0)
		ba, e := NewWrappedAlgebraicOpenShell(ctx, ab.sources[1], ab.sources[0], [2]int{ba0.Loop, ba0.Edge}, [2]int{ba1.Loop, ba1.Edge}, ab.pair.ReversedParameter(), edgecheck.Options{})
		if e != nil {
			t.Fatal(e)
		}
		swap := func(q WrappedCoordinateSeparator) WrappedCoordinateSeparator {
			q.ParameterAxes[0], q.ParameterAxes[1] = q.ParameterAxes[1], q.ParameterAxes[0]
			q.Scale[0], q.Scale[1] = q.Scale[1], q.Scale[0]
			q.Offset[0], q.Offset[1] = q.Offset[1], q.Offset[0]
			return q
		}
		ab, bc = cb, ba
		h = [2]WrappedCoordinateSeparator{swap(h[1]), swap(h[0])}
	}
	s, e := NewWrappedAlgebraicSeparatedChain(ctx, ab, bc, h, geom.SurfaceAffineCoordinateOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e := WriteWrappedAlgebraicSeparatedChainJSON(ctx, &out, s, WrappedSeparatedChainJSONOptions{}); e != nil {
		t.Fatal(e)
	}
	return s, out.Bytes()
}
func TestWrappedAlgebraicSeparatedChainJSON(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for _, tc := range []struct{ negative, reverse bool }{{false, false}, {true, false}, {true, true}} {
		original, raw := separatedChainJSONFixture(t, tc.negative, tc.reverse)
		s, e := ReadWrappedAlgebraicSeparatedChainJSON(ctx, bytes.NewReader(raw), WrappedSeparatedChainJSONOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if len(s.Vertices()) != 8 || len(s.Edges()) != 10 || len(s.Faces()) != 3 || s.SeparationGap().Cmp(big.NewRat(1, 1)) != 0 {
			t.Fatal("native topology/gap")
		}
		native := Solid{vertices: s.Vertices(), edges: s.Edges(), faces: s.Faces(), tolerance: 1e-9}
		r, e := native.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil || !r.Complete {
			t.Fatal(r, e)
		}
		for i := 0; i < 2; i++ {
			a, _ := original.Adjacent(i)
			b, _ := s.Adjacent(i)
			if a.Sides() != b.Sides() || a.Separator().Level.Cmp(b.Separator().Level) != 0 || a.Shell().Pair().ReversedParameter() != b.Shell().Pair().ReversedParameter() {
				t.Fatal("signed original order")
			}
		}
		used := 10 + s.ExactOperations()
		for i, p := range s.adjacent {
			used += p.Shell().ExactOperations() + p.Shell().sources[0].ExactOperations()
			if i == 1 {
				used += p.Shell().sources[1].ExactOperations()
			}
		}
		if q, e := ReadWrappedAlgebraicSeparatedChainJSON(ctx, bytes.NewReader(raw), WrappedSeparatedChainJSONOptions{MaxWork: used - 1}); q != nil || e == nil {
			t.Fatal("cumulative budget")
		}
		var out bytes.Buffer
		if e := WriteWrappedAlgebraicSeparatedChainJSON(ctx, &out, s, WrappedSeparatedChainJSONOptions{}); e != nil || !bytes.Equal(raw, out.Bytes()) {
			t.Fatal("deterministic original-source recipe", e)
		}
		if q, e := ReadWrappedAlgebraicChainShellJSON(ctx, bytes.NewReader(raw), WrappedAlgebraicChainJSONOptions{}); q != nil || e == nil {
			t.Fatal("strict common-chart schema broadened")
		}
	}
}
func TestWrappedAlgebraicSeparatedChainJSONRejects(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	s, raw := separatedChainJSONFixture(t, false, false)
	changes := map[string]func(*wrappedSeparatedChainRecipe){
		"gap":                 func(n *wrappedSeparatedChainRecipe) { n.Gap = "0" },
		"second plane":        func(n *wrappedSeparatedChainRecipe) { n.Separators[1].Level = "1/2" },
		"inconsistent middle": func(n *wrappedSeparatedChainRecipe) { n.Separators[1].Scale[0] = "1" },
		"nonparallel":         func(n *wrappedSeparatedChainRecipe) { n.Separators[1].Axis = 0 },
		"noncanonical":        func(n *wrappedSeparatedChainRecipe) { n.Separators[1].Level = "2/2" },
		"mate":                func(n *wrappedSeparatedChainRecipe) { n.Mates[1].Edges[0][1] = 0 },
		"owner":               func(n *wrappedSeparatedChainRecipe) { n.Witness.Edges[0].Sources[0][0] = 2 },
		"exterior":            func(n *wrappedSeparatedChainRecipe) { n.Witness.Boundary[0][0] = 2 },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			var n wrappedSeparatedChainRecipe
			json.Unmarshal(raw, &n)
			change(&n) // All original charts remain independently valid.
			for _, face := range n.Faces {
				if _, e := ReadWrappedAlgebraicQuadraticFaceJSON(ctx, bytes.NewReader(face), WrappedQuadraticFaceJSONOptions{}); e != nil {
					t.Fatal(e)
				}
			}
			bad, e := json.Marshal(n)
			if e != nil {
				t.Fatal(e)
			}
			if q, e := ReadWrappedAlgebraicSeparatedChainJSON(ctx, bytes.NewReader(bad), WrappedSeparatedChainJSONOptions{}); q != nil || e == nil {
				t.Fatal("inconsistent proof accepted")
			}
		})
	}
	stages := []WrappedSeparatedChainJSONOptions{{MaxWork: 1}, {MaxBytes: int64(len(raw) - 1)}, {Face: WrappedQuadraticFaceJSONOptions{Face: WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{MaxOperations: 1}}}}, {Assembly: edgecheck.Options{MaxWork: 1}}, {Separation: geom.SurfaceAffineCoordinateOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: 1}}}}
	for _, opts := range stages {
		if q, e := ReadWrappedAlgebraicSeparatedChainJSON(ctx, bytes.NewReader(raw), opts); q != nil || e == nil {
			t.Fatal("read stage limit")
		}
		var out bytes.Buffer
		if e := WriteWrappedAlgebraicSeparatedChainJSON(ctx, &out, s, opts); e == nil || out.Len() != 0 {
			t.Fatal("atomic stage publication")
		}
	}
	for _, bad := range [][]byte{append(append([]byte(nil), raw...), []byte(" {}")...), bytes.Replace(raw, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1), bytes.Replace(raw, []byte(`"version":1`), []byte(`"version":1,"proven":true`), 1)} {
		if q, e := ReadWrappedAlgebraicSeparatedChainJSON(ctx, bytes.NewReader(bad), WrappedSeparatedChainJSONOptions{}); q != nil || e == nil {
			t.Fatal("malformed")
		}
	}
	bad := *s
	graph := *s.graph
	graph.edges = s.Edges()
	curve := graph.edges[0].Curve.(wrappedAlgebraicTrimCurve)
	curve.b.X += .125
	graph.edges[0].Curve = curve
	bad.graph = &graph
	var out bytes.Buffer
	if e := WriteWrappedAlgebraicSeparatedChainJSON(ctx, &out, &bad, WrappedSeparatedChainJSONOptions{}); e == nil || out.Len() != 0 {
		t.Fatal("stale native model")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if q, e := ReadWrappedAlgebraicSeparatedChainJSON(canceled, bytes.NewReader(raw), WrappedSeparatedChainJSONOptions{}); q != nil || e == nil {
		t.Fatal("cancel")
	}
}
