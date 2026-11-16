package offset

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
	"plainsolid/mesh"
)

func TestTessellateVerifiedOffsetAtlasStitchesRoundedOriginalEdges(t *testing.T) {
	ctx := context.Background()
	faces := []geom.VerifiedNormalOffsetFace{
		atlasMeshFace(t, big.NewRat(-1, 1), new(big.Rat), false),
		atlasMeshFace(t, new(big.Rat), big.NewRat(4, 1), true),
		atlasMeshFace(t, big.NewRat(4, 1), big.NewRat(5, 1), false),
	}
	joins := []geom.VerifiedNormalOffsetJoin{
		{Faces: [2]int{0, 1}, Edges: [2]geom.SurfaceEdge{geom.SurfaceEdgeUMax, geom.SurfaceEdgeUMin}, ReverseB: true},
		{Faces: [2]int{1, 2}, Edges: [2]geom.SurfaceEdge{geom.SurfaceEdgeUMax, geom.SurfaceEdgeUMin}, ReverseB: true},
	}
	o := VerifiedAtlasMeshOptions{Grid: mesh.NormalOffsetGridOptions{Deflection: .4, MaxDepth: 7}}
	out, err := TessellateVerifiedNormalOffsetAtlas(ctx, faces, joins, o)
	if err != nil || !out.Atlas.Complete || out.Atlas.Closed || len(out.Atlas.JoinProofs) != 2 {
		t.Fatal("atlas does not contain exactly two validated original joins", out.Atlas, err)
	}
	n := out.Segments
	if n <= 1 || out.BoundaryEdges != 8*n || len(out.Mesh.Triangles) != 6*n*n ||
		len(out.Mesh.Vertices) != 3*(n+1)*(n+1)-2*(n+1) {
		t.Fatal("conforming grid did not merge precisely the original seam vertices", n, len(out.Mesh.Vertices),
			len(out.Mesh.Triangles), out.BoundaryEdges)
	}
	for _, plane := range []float64{0, 4} {
		count := 0
		for _, p := range out.Mesh.Vertices {
			if p.X == plane {
				count++
			}
		}
		if count != n+1 {
			t.Fatal("seam has unmerged or extraneous vertices", plane, count)
		}
	}
	target := new(big.Rat).SetFloat64(o.Grid.Deflection)
	if out.DistanceUpperSquared.Cmp(target.Mul(target, target)) > 0 {
		t.Fatal("largest source face exceeds the requested physical bound", out.DistanceUpperSquared)
	}
	o.MaxTriangles = len(out.Mesh.Triangles) - 1
	failed, err := TessellateVerifiedNormalOffsetAtlas(ctx, faces, joins, o)
	if !errors.Is(err, ErrLimit) || len(failed.Mesh.Vertices) != 0 || failed.Atlas.Complete {
		t.Fatal("last missing triangle must refuse atomically", err)
	}
}

func TestTessellateVerifiedOffsetAtlasRejectsRoundedSeparator(t *testing.T) {
	ctx := context.Background()
	seam := big.NewRat(1, 3)
	faces := []geom.VerifiedNormalOffsetFace{
		atlasMeshFace(t, new(big.Rat), seam, false),
		atlasMeshFace(t, seam, big.NewRat(1, 1), false),
	}
	joins := []geom.VerifiedNormalOffsetJoin{
		{Faces: [2]int{0, 1}, Edges: [2]geom.SurfaceEdge{geom.SurfaceEdgeUMax, geom.SurfaceEdgeUMin}},
	}
	if proof, err := geom.CertifyVerifiedNormalOffsetAtlas(ctx, faces, joins, geom.VerifiedNormalOffsetAtlasOptions{}); err != nil || !proof.Complete {
		t.Fatal("the exact source seam should be valid", proof, err)
	}
	out, err := TessellateVerifiedNormalOffsetAtlas(ctx, faces, joins,
		VerifiedAtlasMeshOptions{Grid: mesh.NormalOffsetGridOptions{Deflection: .7}})
	if !errors.Is(err, ErrPrecision) || len(out.Mesh.Triangles) != 0 || out.DistanceUpperSquared != nil {
		t.Fatal("a rounded-away separator cannot be silently welded", out, err)
	}
}

func atlasMeshFace(t *testing.T, a, b *big.Rat, reversed bool) geom.VerifiedNormalOffsetFace {
	t.Helper()
	ctx := context.Background()
	var net [][4]*big.Rat
	for _, x := range []*big.Rat{a, b} {
		for i := range 3 {
			y := int64(i) - 1
			if reversed {
				y = -y
			}
			net = append(net, [4]*big.Rat{x, big.NewRat(y, 1), big.NewRat([]int64{1, -1, 1}[i], 1), big.NewRat(1, 1)})
		}
	}
	p, err := geom.NewExactHomogeneousBezierSurface(ctx, 1, 2, net, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	dir := int64(1)
	if reversed {
		dir = -1
	}
	verified, _, err := geom.NewVerifiedNormalOffsetPatch(ctx, p, big.NewRat(dir, 10),
		geom.NormalOffsetProjection{Axes: [2]int{0, 1}, Reversed: [2]bool{false, reversed}}, geom.NormalOffsetCoverageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return geom.VerifiedNormalOffsetFace{Patch: verified, ReverseWinding: reversed}
}
