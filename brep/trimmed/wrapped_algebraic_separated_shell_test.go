//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

func wrappedSeparatedAlgebraicFixture(t *testing.T) (*WrappedAlgebraicOpenShell, WrappedCoordinateSeparator) {
	t.Helper()
	ctx := context.Background()
	base, h := wrappedSeparatedFixture(t, false, false)
	aLoops, bLoops := base.sources[0].OriginalLoops()[0], base.sources[1].OriginalLoops()[0]
	q, e := NewExactQuadraticUV([3][3]*big.Rat{{big.NewRat(1, 4), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 4), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(-1, 4), new(big.Rat), big.NewRat(1, 1)}})
	if e != nil {
		t.Fatal(e)
	}
	aLoops[2] = q
	bq := reversePlaneQuadratic(q).HomogeneousControls()
	for i := range bq {
		bq[i][0].Add(bq[i][0], new(big.Rat).Mul(big.NewRat(4, 1), bq[i][2]))
	}
	bLoops[0], e = NewExactQuadraticUV(bq)
	if e != nil {
		t.Fatal(e)
	}
	opts := WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}}
	a, e := NewWrappedAlgebraicQuadraticFace(ctx, base.sources[0].NativeFace().Face.Surface.(*geom.WrappedSurfacePatch), aLoops, nil, opts)
	if e != nil {
		t.Fatal(e)
	}
	b, e := NewWrappedAlgebraicQuadraticFace(ctx, base.sources[1].NativeFace().Face.Surface.(*geom.WrappedSurfacePatch), bLoops, nil, opts)
	if e != nil {
		t.Fatal(e)
	}
	pair, e := NewWrappedAlgebraicOpenShell(ctx, a, b, [2]int{0, 2}, [2]int{0, 0}, true, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	return pair, h
}
func separatedShellJSONFixture(t *testing.T) (*WrappedAlgebraicSeparatedShell, []byte) {
	t.Helper()
	pair, h := wrappedSeparatedAlgebraicFixture(t)
	s, e := NewWrappedAlgebraicSeparatedShell(context.Background(), pair, h, geom.SurfaceAffineCoordinateOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e := WriteWrappedAlgebraicSeparatedShellJSON(context.Background(), &out, s, WrappedSeparatedShellJSONOptions{}); e != nil {
		t.Fatal(e)
	}
	return s, out.Bytes()
}
func TestWrappedAlgebraicSeparatedShellJSON(t *testing.T) {
	ctx := context.Background()
	source, raw := separatedShellJSONFixture(t)
	s, e := ReadWrappedAlgebraicSeparatedShellJSON(ctx, bytes.NewReader(raw), WrappedSeparatedShellJSONOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Vertices()) != 6 || len(s.Edges()) != 7 || len(s.Faces()) != 2 || len(s.Boundary()) != 6 || s.Separation().Sides() != ([2]int{-1, 1}) {
		t.Fatal("embedded native disk")
	}
	if s.Faces()[0].Surface.(*geom.WrappedSurfacePatch).Domain() == s.Faces()[1].Surface.(*geom.WrappedSurfacePatch).Domain() {
		t.Fatal("different original charts lost")
	}
	native := Solid{vertices: s.Vertices(), edges: s.Edges(), faces: s.Faces(), tolerance: 1e-9}
	r, e := native.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !r.Complete {
		t.Fatal(r, e)
	}
	for _, pair := range []*WrappedAlgebraicOpenShell{source.PairShell(), s.PairShell()} {
		composition, _ := pair.Pair().Composition(0)
		piece, _ := composition.Piece(0)
		root, _ := piece.Model.EndpointRoot(1)
		kind, e := root.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
		if e != nil || kind.Kind != geom.ExactRootKindIrrational {
			t.Fatal("original irrational interior seam identity", kind, e)
		}
		owner, _ := pair.Pair().Owner(1)
		if owner.UV[0][0].Cmp(big.NewRat(15, 4)) != 0 {
			t.Fatal("original lifted mate")
		}
	}
	owners, e := s.EdgeSources(2)
	if e != nil || len(owners) != 2 || owners[0].Face != 0 || owners[1].Face != 1 {
		t.Fatal("shared native edge")
	}
	var out bytes.Buffer
	if e := WriteWrappedAlgebraicSeparatedShellJSON(ctx, &out, s, WrappedSeparatedShellJSONOptions{}); e != nil || !bytes.Equal(raw, out.Bytes()) {
		t.Fatal("deterministic roundtrip", e)
	}
	if p, e := NewWrappedAlgebraicSeparatedShell(ctx, s.PairShell(), s.Separation().Separator(), geom.SurfaceAffineCoordinateOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: s.ExactOperations() - 1}}); p != nil || e == nil {
		t.Fatal("cumulative all-use bound")
	}
	if p, e := ReadWrappedAlgebraicOpenShellJSON(ctx, bytes.NewReader(raw), WrappedAlgebraicOpenShellJSONOptions{}); p != nil || e == nil {
		t.Fatal("old pair schema broadened")
	}
}
func TestWrappedAlgebraicSeparatedShellJSONRejects(t *testing.T) {
	ctx := context.Background()
	s, raw := separatedShellJSONFixture(t)
	changes := map[string]func(*wrappedSeparatedShellRecipe){
		"level":        func(n *wrappedSeparatedShellRecipe) { n.Separator.Level = "1/3" },
		"scale":        func(n *wrappedSeparatedShellRecipe) { n.Separator.Scale[1] = "1" },
		"wrapped axis": func(n *wrappedSeparatedShellRecipe) { n.Separator.ParameterAxes[1] = 0 },
		"noncanonical": func(n *wrappedSeparatedShellRecipe) { n.Separator.Scale[1] = "4/2" },
		"mate": func(n *wrappedSeparatedShellRecipe) {
			var p wrappedAlgebraicShellRecipe
			json.Unmarshal(n.Pair, &p)
			p.Mate[1][1] = 1
			n.Pair, _ = json.Marshal(p)
		},
		"owner": func(n *wrappedSeparatedShellRecipe) {
			var p wrappedAlgebraicShellRecipe
			json.Unmarshal(n.Pair, &p)
			p.Witness.Edges[2].Sources[1][0] = 0
			n.Pair, _ = json.Marshal(p)
		},
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			var n wrappedSeparatedShellRecipe
			json.Unmarshal(raw, &n)
			change(&n)
			bad, e := json.Marshal(n)
			if e != nil {
				t.Fatal(e)
			}
			if p, e := ReadWrappedAlgebraicSeparatedShellJSON(ctx, bytes.NewReader(bad), WrappedSeparatedShellJSONOptions{}); p != nil || e == nil {
				t.Fatal("mutation accepted")
			}
		})
	}
	for _, opts := range []WrappedSeparatedShellJSONOptions{{MaxWork: 1}, {MaxBytes: int64(len(raw) - 1)}, {Separation: geom.SurfaceAffineCoordinateOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: 1}}}} {
		if p, e := ReadWrappedAlgebraicSeparatedShellJSON(ctx, bytes.NewReader(raw), opts); p != nil || e == nil {
			t.Fatal("read bound")
		}
		var out bytes.Buffer
		if e := WriteWrappedAlgebraicSeparatedShellJSON(ctx, &out, s, opts); e == nil || out.Len() != 0 {
			t.Fatal("atomic writer")
		}
	}
	for _, bad := range [][]byte{append(append([]byte(nil), raw...), []byte(" {}")...), bytes.Replace(raw, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1), bytes.Replace(raw, []byte(`"version":1`), []byte(`"version":1,"proven":true`), 1)} {
		if p, e := ReadWrappedAlgebraicSeparatedShellJSON(ctx, bytes.NewReader(bad), WrappedSeparatedShellJSONOptions{}); p != nil || e == nil {
			t.Fatal("malformed")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if p, e := ReadWrappedAlgebraicSeparatedShellJSON(canceled, bytes.NewReader(raw), WrappedSeparatedShellJSONOptions{}); p != nil || e == nil {
		t.Fatal("cancel")
	}
}

func TestWrappedAlgebraicSeparatedShellReversedJSON(t *testing.T) {
	ctx := context.Background()
	pair, h := wrappedSeparatedFixture(t, true, false)
	s, e := NewWrappedAlgebraicSeparatedShell(ctx, pair, h, geom.SurfaceAffineCoordinateOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e := WriteWrappedAlgebraicSeparatedShellJSON(ctx, &out, s, WrappedSeparatedShellJSONOptions{}); e != nil {
		t.Fatal(e)
	}
	restored, e := ReadWrappedAlgebraicSeparatedShellJSON(ctx, bytes.NewReader(out.Bytes()), WrappedSeparatedShellJSONOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if restored.PairShell().Pair().ReversedParameter() || !restored.Faces()[1].Reversed || restored.Separation().Separator().Scale[1].Cmp(big.NewRat(-2, 1)) != 0 || restored.Separation().Sides() != ([2]int{-1, 1}) {
		t.Fatal("original signed chart or mate direction lost")
	}
}
