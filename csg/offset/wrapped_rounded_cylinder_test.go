package offset

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestWrappedRoundedCylinderCompleteOffset(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 3, 5, o)
	if e != nil {
		t.Fatal(e)
	}
	stock, _, e := trimmed.NewWrappedCylinder(ctx, w, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	// Reindex the three sharp source faces so ancestry cannot rely on
	// constructor order.
	f := stock.Faces()
	stock, e = trimmed.NewSolid(stock.Vertices(), stock.Edges(), []trimmed.Face{f[2], f[0], f[1]}, stock.Tolerance())
	if e != nil {
		t.Fatal(e)
	}
	const d = .25
	out, h, e := WrappedRoundedCylinder(ctx, stock, d, o)
	if e != nil || len(h.SourceFaces) != 5 || len(h.SourceFaces[h.ResultBlends[0]]) != 2 ||
		h.SourceFaces[h.ResultBlends[0]][0] != 1 || h.SourceFaces[h.ResultBlends[0]][1] != 2 ||
		h.SourceFaces[h.ResultCaps[1]][0] != 0 {
		t.Fatal(h, e)
	}
	// Independent volume by Steiner expansion of the original cylinder:
	// V + area*d + integrated mean curvature*d² + 4*pi*d³/3.
	mass, e := out.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-7})
	want := math.Pi * (45 + 48*d + (5+3*math.Pi)*d*d + 4*d*d*d/3)
	if e != nil || math.Abs(mass.Volume-want) > 1e-7 {
		t.Fatal(mass, want, e)
	}
	restored, restoredHistory, e := WrappedRoundedCylinder(ctx, out, -.25, o)
	if e != nil || restoredHistory.ResultBlends != [2]int{-1, -1} {
		t.Fatal("exact dilation/erosion inverse", restoredHistory, e)
	}
	_, restoredProof, e := restored.ValidateCanonicalWrappedCylinder(ctx, o)
	if e != nil || restoredProof.Radius.Cmp(big.NewRat(3, 1)) != 0 || restoredProof.LowerZ.Sign() != 0 || restoredProof.UpperZ.Cmp(big.NewRat(5, 1)) != 0 {
		t.Fatal("restored original cylinder", restoredProof, e)
	}
	if _, _, e := WrappedRoundedCylinder(ctx, out, math.Nextafter(-.25, math.Inf(-1)), o); !errors.Is(e, geom.ErrDomain) {
		t.Fatal("negative blend continuation should be rejected", e)
	}
	sharp, hs, e := WrappedRoundedCylinder(ctx, stock, -.25, o)
	if e != nil || hs.ResultBlends != [2]int{-1, -1} {
		t.Fatal(hs, e)
	}
	_, ps, e := sharp.ValidateCanonicalWrappedCylinder(ctx, o)
	if e != nil || ps.Radius.Cmp(big.NewRat(11, 4)) != 0 || ps.LowerZ.Cmp(big.NewRat(1, 4)) != 0 || ps.UpperZ.Cmp(big.NewRat(19, 4)) != 0 {
		t.Fatal(ps, e)
	}
	// Existing lower blend changes radius around the same rolling center.
	// The top starts sharp and gets a new face on outward offset.
	original, history, e := trimmed.FilletWrappedCylinderRims(ctx, stock, .5, 0, o)
	if e != nil {
		t.Fatal(e)
	}
	q, hq, e := WrappedRoundedCylinder(ctx, original, .25, o)
	if e != nil || len(hq.SourceFaces[hq.ResultBlends[0]]) != 1 || hq.SourceFaces[hq.ResultBlends[0]][0] != history.Blends[0] {
		t.Fatal(hq, e)
	}
	p, e := q.ValidateWrappedCylinderRimFillet(ctx, o)
	if e != nil || p.Fillets[0].Cmp(big.NewRat(3, 4)) != 0 || p.Fillets[1].Cmp(big.NewRat(1, 4)) != 0 {
		t.Fatal(p, e)
	}
	// Normal displacement from the source blend at a nonbinary
	// meridian parameter is exactly d, even though both points are rational.
	faces, moved := original.Faces(), q.Faces()
	for _, uv := range [][2]*big.Rat{{big.NewRat(1, 3), big.NewRat(10, 3)}, {big.NewRat(7, 3), big.NewRat(11, 3)}} {
		a, e := faces[history.Blends[0]].Surface.(*geom.WrappedSurfacePatch).EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		b, e := moved[hq.ResultBlends[0]].Surface.(*geom.WrappedSurfacePatch).EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		var n big.Rat
		for i := range a.Point {
			v := new(big.Rat).Sub(a.Point[i], b.Point[i])
			n.Add(&n, v.Mul(v, v))
		}
		if n.Cmp(big.NewRat(1, 16)) != 0 {
			t.Fatal("wrong exact offset", &n)
		}
	}
}
