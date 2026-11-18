//go:build research

package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestMixedOriginalUniqueRoofInterior(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, b, p, o := mixedWallFixture(t)
	for _, s := range []Solid{a, b} {
		if q, e := proveMixedRoofBox(ctx, s, 1, 8000000); e != nil {
			t.Fatal("graph box", e)
		} else if q.floor == nil {
			t.Fatal("missing floor")
		}
	}
	section, e := SectionSplineShellsMixedAlgebraic(ctx, a, b, p, o)
	if e != nil {
		t.Fatal(e)
	}
	bo := AlgebraicShellBooleanOptions{}
	bo.Imprint.Exact.MaxOperations = 128000000
	solids, e := section.CombineAlgebraicOperations(ctx, []parametric.BooleanOp{parametric.Intersection}, bo)
	if e != nil {
		t.Fatal(e)
	}
	body := solids[0]
	n, e := body.TessellateEmbeddedBoundaryCertified(ctx, AlgebraicShellBoundaryEmbeddingOptions{Boundary: AlgebraicShellBoundaryOptions{Deflection: .02, MaxOperations: 128000000, AutoSectionProjection: true, PlanarSectionFallback: true}})
	if e != nil {
		t.Fatal(e)
	}
	face := -1
	for i, f := range body.Faces() {
		if f.Operand == 1 && f.Original == 1 {
			face = i
		}
	}
	options := MixedRoofInteriorOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: .2}}
	r, e := n.TessellateUniqueRoofInteriorCertified(ctx, face, 1, options)
	if e != nil {
		t.Fatal("roof", e)
	}
	m := r.Mesh()
	uv := r.ExactUV()
	bounds, clearance := r.TriangleBounds()
	if len(m.Triangles) == 0 || len(bounds) != len(m.Triangles) || len(clearance) != len(bounds) || r.Complete() || r.OmittedTriangles() == 0 {
		t.Fatal("partial scope")
	}
	for i, v := range n.Vertices() {
		if m.Vertices[i] != v {
			t.Fatal("moved boundary")
		}
	}
	// Independent analytic roof difference: (X-1/4)(X-3/4)+Y(1-Y)/64.
	// Check all exact triangle vertices against the original retained B roof.
	interiorNodes := map[int]bool{}
	for i, tri := range m.Triangles {
		if clearance[i].Sign() <= 0 || bounds[i].Cmp(big.NewRat(1, 25)) > 0 {
			t.Fatal("invalid proof bound")
		}
		for _, id := range tri {
			if id >= len(n.Vertices()) {
				interiorNodes[id] = true
			}
			x, y := uv[id][0], uv[id][1]
			gap := new(big.Rat).Mul(new(big.Rat).Sub(x, big.NewRat(1, 4)), new(big.Rat).Sub(x, big.NewRat(3, 4)))
			gap.Add(gap, new(big.Rat).Quo(new(big.Rat).Mul(y, new(big.Rat).Sub(big.NewRat(1, 1), y)), big.NewRat(64, 1)))
			if gap.Sign() >= 0 || x.Sign() <= 0 || x.Cmp(big.NewRat(1, 1)) >= 0 || y.Sign() <= 0 || y.Cmp(big.NewRat(1, 1)) >= 0 {
				t.Fatal("outside actual retained roof")
			}
		}
	}
	if len(interiorNodes) == 0 {
		t.Fatal("no genuine original graph interior evaluations")
	}
	options.RequireComplete = true
	if got, e := n.TessellateUniqueRoofInteriorCertified(ctx, face, 1, options); got != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("unproved collar published", e)
	}
	options.RequireComplete = false
	options.Proof.MaxOperations = 1
	if got, e := n.TessellateUniqueRoofInteriorCertified(ctx, face, 1, options); got != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("budget", e)
	}
	options.Proof.MaxOperations = 0
	options.Proof.MaxTriangles = 1
	if got, e := n.TessellateUniqueRoofInteriorCertified(ctx, face, 1, options); got != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("ignored triangle budget", e)
	}
	options.Proof.MaxTriangles = 0
	if got, e := n.TessellateUniqueRoofInteriorCertified(ctx, face, 0, options); got != nil || e == nil {
		t.Fatal("wrong opposite roof accepted")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if got, e := n.TessellateUniqueRoofInteriorCertified(canceled, face, 1, options); got != nil || !errors.Is(e, context.Canceled) {
		t.Fatal("cancellation", e)
	}
	bounds[0].SetInt64(999)
	copyUV := r.ExactUV()
	id := m.Triangles[0][0]
	copyUV[id][0].SetInt64(999)
	fresh, _ := r.TriangleBounds()
	if fresh[0].Cmp(big.NewRat(999, 1)) == 0 || r.ExactUV()[id][0].Cmp(big.NewRat(999, 1)) == 0 {
		t.Fatal("mutable certificate")
	}
	ops, checks, pairs := r.Work()
	t.Log("roof", face, "triangles", len(m.Triangles), "omitted", r.OmittedTriangles(), "operations", ops, "triangle checks", checks, "contact pairs", pairs)
}
