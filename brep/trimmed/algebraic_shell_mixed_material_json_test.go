//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestMixedMaterialPersistenceAndVolume(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, b, p, o := mixedWallFixture(t)
	section, err := SectionSplineShellsMixedAlgebraic(ctx, a, b, p, o)
	if err != nil {
		t.Fatal(err)
	}
	bc := AlgebraicShellBooleanOptions{}
	bc.Imprint.Exact.MaxOperations = 128000000
	bodies, err := section.CombineAlgebraicOperations(ctx, []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference}, bc)
	if err != nil {
		t.Fatal(err)
	}
	// Independently integrate XY/64 over the two stock footprints. The
	// negative part of the roof difference contributes -(4/3) integral_0^1
	// [1/16-y(1-y)/64]^(3/2)dy on their unit-square overlap.
	common := .5 + 1.0/256 - 77.0/6144 - (225.0/8192)*math.Asinh(1/math.Sqrt(15))
	av := 1.25 + 5.0/1024
	bv := .625 + 5.0/1024 + 5.0/192 + 115.0/49152
	for i, want := range []float64{common, av + bv - common, av - common} {
		bound, e := bodies[i].BoundVolume(ctx, AlgebraicShellVolumeOptions{MaxDepth: 6, TubeSubdivisions: 8})
		if e != nil {
			t.Fatal("volume", i, e)
		}
		lo, _ := bound.Volume.Lower.Float64()
		hi, _ := bound.Volume.Upper.Float64()
		t.Log("volume", i, lo, hi, "analytic", want, "cells", bound.VisitedCells, "ambiguous", bound.AmbiguousCells)
		if lo <= 0 || lo > want || hi < want || hi-lo > 1.1 {
			t.Fatal("independent integral outside certificate", i, lo, hi, want)
		}
	}
	// Check original operand UV order independently of material integration,
	// including the nonbinary target free coordinate and the reversed owner map.
	for _, br := range section.source.branches {
		if br.rational == nil {
			continue
		}
		cover, e := rationalShellBranchCover(ctx, br, 3, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for j := 0; j < br.rational.PieceCount(); j++ {
			d, _ := br.rational.Piece(j)
			for _, t0 := range []*big.Rat{d.Free.Lower, new(big.Rat).Quo(new(big.Rat).Add(d.Free.Lower, d.Free.Upper), big.NewRat(2, 1)), d.Free.Upper} {
				event, e := br.rational.EvaluatePiece(ctx, j, t0, geom.TubeAlgebraicGraphEndpointOptions{})
				if e != nil {
					t.Fatal(e)
				}
				for op := 0; op < 2; op++ {
					owner := op
					if br.rationalSource == 1 {
						owner = 1 - op
					}
					var uv [2]*big.Rat
					for axis := 0; axis < 2; axis++ {
						value, e := event.Parameters[2*owner+axis].Classify(ctx, geom.ExactCurvePlaneEventOptions{})
						if e != nil || value.Rational == nil {
							t.Fatal("not original rational chart", e)
						}
						uv[axis] = value.Rational
					}
					covered := false
					for _, box := range cover[op] {
						covered = covered || (uv[0].Cmp(ratInset(box.Min.X)) >= 0 && uv[0].Cmp(ratInset(box.Max.X)) <= 0 && uv[1].Cmp(ratInset(box.Min.Y)) >= 0 && uv[1].Cmp(ratInset(box.Max.Y)) <= 0)
					}
					if !covered {
						t.Fatal("lost original chart cover", op, uv)
					}
				}
			}
		}
	}
	var saved bytes.Buffer
	if err := WriteMixedAlgebraicShellMaterialJSON(&saved, bodies[0]); err != nil {
		t.Fatal(err)
	}
	reload, err := ReadMixedAlgebraicShellMaterialJSON(ctx, bytes.NewReader(saved.Bytes()), JSONLimits{}, o, bc)
	if err != nil {
		t.Fatal("reload", err)
	}
	before, _ := bodies[0].Topology()
	after, _ := reload.Topology()
	if before != after {
		t.Fatal("changed topology")
	}
	h1, e1 := bodies[0].History()
	h2, e2 := reload.History()
	if e1 != nil || e2 != nil || len(h1.Faces) != len(h2.Faces) || len(h1.Edges) != len(h2.Edges) {
		t.Fatal("reload changed Boolean history inventory", e1, e2)
	}
	for i, a := range h1.Edges {
		b := h2.Edges[i]
		if a.Operand != b.Operand || a.OriginalEdge != b.OriginalEdge || a.Branch != b.Branch || a.Piece != b.Piece || a.Faces != b.Faces || a.Reversed != b.Reversed {
			t.Fatal("reload changed original edge ancestry", i, a, b)
		}
		for k, root := range [2]*geom.ExactAlgebraicRoot{a.Lower, a.Upper} {
			other := [2]*geom.ExactAlgebraicRoot{b.Lower, b.Upper}[k]
			if (root == nil) != (other == nil) {
				t.Fatal("reload changed original split identity", i, k)
			}
			if root != nil {
				v, e := root.Compare(ctx, other, geom.ExactCurvePlaneEventOptions{})
				if e != nil || !v.Proven || v.Order != 0 {
					t.Fatal("reload changed exact original split parameter", i, k, v, e)
				}
			}
		}
	}
	restricted := 0
	for i, edge := range reload.Edges() {
		if edge.SectionPiece < 0 {
			continue
		}
		piece, e := reload.SectionPiece(i)
		if e != nil {
			t.Fatal(e)
		}
		if piece.Restricted != nil {
			restricted++
			d, _ := piece.Restricted.Piece(piece.Index)
			if piece.Index == 1 && d.Free.Lower.Cmp(big.NewRat(109, 130)) != 0 {
				t.Fatal("rounded retained domain")
			}
		}
	}
	if restricted != 2 {
		t.Fatal("missing restricted model pieces")
	}
	for i, owner := range reload.Faces() {
		f, e := reload.OriginalFace(i)
		if e != nil {
			t.Fatal(e)
		}
		actual, e := parametric.EncodeSurface(f.Surface)
		if e != nil {
			t.Fatal(e)
		}
		original, e := parametric.EncodeSurface([2]Solid{a, b}[owner.Operand].faces[owner.Original].Surface)
		if e != nil || !bytes.Equal(actual, original) {
			t.Fatal("changed original support history", e)
		}
	}
	for _, row := range []struct {
		point geom.Vec3
		want  parametric.Location
	}{
		{geom.Vec3{X: .5, Y: .5, Z: -.25}, parametric.Inside},
		{geom.Vec3{X: .5, Y: .5, Z: -.025}, parametric.Outside},
		{geom.Vec3{X: 1, Y: 1, Z: -19.0 / 128}, parametric.Boundary},
	} {
		location, e := reload.LocateMaterial(ctx, row.point, SplineShellBooleanOptions{})
		if e != nil || location != row.want {
			t.Fatal("reloaded original material", row, location, e)
		}
	}
	for _, change := range []string{"operation", "owner", "missing branch"} {
		var q nativeMixedMaterial
		if e := json.Unmarshal(saved.Bytes(), &q); e != nil {
			t.Fatal(e)
		}
		switch change {
		case "operation":
			q.Operation = parametric.Union
		case "owner":
			q.Faces[0].Original++
		case "missing branch":
			var source nativeMixedShellSection
			if e := json.Unmarshal(q.Section, &source); e != nil {
				t.Fatal(e)
			}
			source.Wires[0] = source.Wires[0][:len(source.Wires[0])-1]
			q.Section, _ = json.Marshal(source)
		}
		data, _ := json.Marshal(q)
		got, e := ReadMixedAlgebraicShellMaterialJSON(ctx, bytes.NewReader(data), JSONLimits{}, o, bc)
		if got != nil || e == nil {
			t.Fatal("accepted changed witness", change, e)
		}
	}
	for _, limits := range []JSONLimits{{MaxBytes: 10}, {MaxEntities: 1}} {
		got, e := ReadMixedAlgebraicShellMaterialJSON(ctx, bytes.NewReader(saved.Bytes()), limits, o, bc)
		if got != nil || !errors.Is(e, geom.ErrConvergence) {
			t.Fatal("read budget", e)
		}
	}
	lowWork := AlgebraicShellVolumeOptions{}
	lowWork.Shells.Embedding.Patches.MaxOperations = 1
	if bound, e := bodies[0].BoundVolume(ctx, lowWork); !errors.Is(e, geom.ErrConvergence) || bound.Volume.Lower != nil {
		t.Fatal("volume work cap", e)
	}
	limited, e := bodies[0].BoundVolume(ctx, AlgebraicShellVolumeOptions{MaxCells: 1})
	if !errors.Is(e, geom.ErrConvergence) || limited.Volume.Lower != nil || limited.Volume.Upper != nil {
		t.Fatal("partial volume published", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if got, e := ReadMixedAlgebraicShellMaterialJSON(canceled, bytes.NewReader(saved.Bytes()), JSONLimits{}, o, bc); got != nil || !errors.Is(e, context.Canceled) {
		t.Fatal("canceled decode", e)
	}
	if got, e := bodies[0].BoundVolume(canceled, AlgebraicShellVolumeOptions{}); !errors.Is(e, context.Canceled) || got.Volume.Lower != nil {
		t.Fatal("canceled volume", e)
	}
	var output bytes.Buffer
	if e := WriteMixedAlgebraicShellMaterialJSON(&output, &MixedAlgebraicShellBoolean{}); !errors.Is(e, geom.ErrInvalid) || output.Len() != 0 {
		t.Fatal("forged material write", e)
	}
}

func TestMixedMaterialJSONStructuralRejection(t *testing.T) {
	for _, data := range []string{`{"version":2,"kind":"trimmed-mixed-algebraic-material"}`, `{"version":1,"kind":"trimmed-mixed-algebraic-material","operation":255,"section":{}}`, `{"version":1,"kind":"trimmed-mixed-algebraic-material","complete":true}`, `{} {}`} {
		out, err := ReadMixedAlgebraicShellMaterialJSON(context.Background(), bytes.NewBufferString(data), JSONLimits{}, MixedAlgebraicShellSectionOptions{}, AlgebraicShellBooleanOptions{})
		if out != nil || err == nil {
			t.Fatal("malformed recipe accepted", data)
		}
	}
}
