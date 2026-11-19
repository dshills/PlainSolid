//go:build research

package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

func TestMixedAllRoofCollarsAndClosedMesh(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, b, p, o := mixedWallFixture(t)
	section, e := SectionSplineShellsMixedAlgebraic(ctx, a, b, p, o)
	if e != nil {
		t.Fatal(e)
	}
	bo := AlgebraicShellBooleanOptions{}
	bo.Imprint.Exact.MaxOperations = 128000000
	bodies, e := section.CombineAlgebraicOperations(ctx, []parametric.BooleanOp{parametric.Intersection}, bo)
	if e != nil {
		t.Fatal(e)
	}
	body := bodies[0]
	n, e := body.TessellateEmbeddedBoundaryCertified(ctx, AlgebraicShellBoundaryEmbeddingOptions{Boundary: AlgebraicShellBoundaryOptions{Deflection: .02, MaxOperations: 128000000, AutoSectionProjection: true, PlanarSectionFallback: true}})
	if e != nil {
		t.Fatal(e)
	}
	full := mesh.Mesh{Vertices: n.Vertices()}
	roofCount, collars, bulk := 0, 0, 0
	var roofs []*CertifiedMixedRoofFragment
	for face, f := range body.Faces() {
		if f.Original != 1 {
			continue
		}
		options := MixedRoofCollarOptions{Roof: MixedRoofInteriorOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: .2}}, Width: .04}
		r, e := n.TessellateRoofFragmentCertified(ctx, face, 1, options)
		if e != nil {
			t.Fatal("roof", face, e)
		}
		roofCount++
		roofs = append(roofs, r)
		if r.SourceImageSquaredDistance().Cmp(new(big.Rat).Mul(ratInset(.2), ratInset(.2))) > 0 {
			t.Fatal("roof image distance")
		}
		sums := map[int]*big.Rat{}
		for _, cell := range r.ClearanceCells() {
			if sums[cell.Triangle] == nil {
				sums[cell.Triangle] = new(big.Rat)
			}
			sums[cell.Triangle].Add(sums[cell.Triangle], roofUVArea(cell.UV))
			if cell.VerticalClearance.Sign() > 0 {
				bulk++
				continue
			}
			collars++
			cert, e := n.EdgeCertificate(cell.NativeEdge)
			if e != nil || cert.Kind != cell.CurveKind {
				t.Fatal("lost original curve owner", e)
			}
			path := n.Edges()[cell.NativeEdge]
			x, y := n.Vertices()[path[cell.Chord]], n.Vertices()[path[cell.Chord+1]]
			for _, p := range cell.UV {
				distance := roofPointSegmentDistance(p, [2]*big.Rat{ratInset(x.X), ratInset(x.Y)}, [2]*big.Rat{ratInset(y.X), ratInset(y.Y)})
				if distance.Cmp(cell.ChordSquaredDistance) > 0 || cell.ChordSquaredDistance.Cmp(ratInset(.04*.04)) > 0 {
					t.Fatal("unproved collar")
				}
			}
			if cell.SourceTrimSquaredDistance.Sign() <= 0 {
				t.Fatal("missing source trim transport")
			}
		}
		m := r.Mesh()
		exactUV := r.ExactUV()
		for index, tri := range m.Triangles {
			area := roofUVArea([3][2]*big.Rat{exactUV[tri[0]], exactUV[tri[1]], exactUV[tri[2]]})
			if sums[index] == nil || sums[index].Cmp(area) != 0 {
				t.Fatal("incomplete clearance subdivision")
			}
		}

		mapping := make([]int, len(m.Vertices))
		for i, v := range m.Vertices {
			if i < len(n.Vertices()) {
				mapping[i] = i
				if full.Vertices[i] != v {
					t.Fatal("moved shared boundary")
				}
			} else {
				mapping[i] = len(full.Vertices)
				full.Vertices = append(full.Vertices, v)
			}
		}
		for _, tri := range m.Triangles {
			full.Triangles = append(full.Triangles, [3]int{mapping[tri[0]], mapping[tri[1]], mapping[tri[2]]})
		}
		t.Log("roof", face, "triangles", len(m.Triangles), "clearance cells", len(r.ClearanceCells()))
	}
	var proposals []MixedPlanarFragmentProposal
	for id, f := range body.Faces() {
		if f.Original == 1 {
			continue
		}
		q := MixedPlanarFragmentProposal{Face: id}
		switch f.Original {
		case 0:
			z := []float64{-1, -.5}[f.Operand]
			q.Plane = mixedCoordinatePlane(2, z)
			q.ProjectionAxes = [2]int{0, 1}
		case 2, 3:
			x := [][2]float64{{-.125, 1.125}, {0, 1}}[f.Operand][f.Original-2]
			q.Plane = mixedCoordinatePlane(0, x)
			q.ProjectionAxes = [2]int{1, 2}
		case 4, 5:
			y := [][2]float64{{0, 1}, {-.125, 1.125}}[f.Operand][f.Original-4]
			q.Plane = mixedCoordinatePlane(1, y)
			q.ProjectionAxes = [2]int{0, 2}
		}
		proposals = append(proposals, q)
	}
	planar, e := n.FillPlanarFragmentsCertified(ctx, proposals, MixedAffineFragmentFillOptions{})
	if e != nil {
		t.Fatal(e)
	}
	full.Triangles = append(full.Triangles, planar.Mesh().Triangles...)
	if roofCount != 3 || collars == 0 || bulk == 0 {
		t.Fatal("incomplete roof coverage")
	}
	if e := full.ValidateSolid(); e != nil {
		t.Fatal("manifold", e)
	}
	ids := make([]int, len(full.Vertices))
	for i := range ids {
		ids[i] = i
	}
	if e := validateQuadraticJunctionFans(full, ids); e != nil {
		t.Fatal("vertex fan", e)
	}

	complete, e := n.AssembleCompleteMeshCertified(ctx, planar, roofs, MixedCompleteMeshOptions{})
	if e != nil {
		t.Fatal("assembly", e)
	}
	if !reflect.DeepEqual(complete.Mesh(), full) || complete.SourceMaterial() != body || complete.DisplaySignedVolume().Sign() <= 0 || len(complete.TriangleFaces()) != len(full.Triangles) {
		t.Fatal("lost original material binding")
	}
	work := complete.Work()
	if work.StrictMaterialCells != bulk || work.BoundaryCollarCells != collars || work.AssemblyContactPairs <= 0 {
		t.Fatal("lost proof counts")
	}
	if got, e := n.AssembleCompleteMeshCertified(ctx, planar, roofs[:2], MixedCompleteMeshOptions{}); got != nil || e == nil {
		t.Fatal("missing face accepted")
	}
	duplicate := append(append([]*CertifiedMixedRoofFragment(nil), roofs...), roofs[0])
	if got, e := n.AssembleCompleteMeshCertified(ctx, planar, duplicate, MixedCompleteMeshOptions{}); got != nil || e == nil {
		t.Fatal("duplicate face accepted")
	}
	copyNetwork := *n
	if got, e := copyNetwork.AssembleCompleteMeshCertified(ctx, planar, roofs, MixedCompleteMeshOptions{}); got != nil || e == nil {
		t.Fatal("rebound network accepted")
	}
	for _, limits := range []MixedCompleteMeshOptions{{MaxTriangles: 1}, {MaxPairs: 1}} {
		if got, e := n.AssembleCompleteMeshCertified(ctx, planar, roofs, limits); got != nil || e == nil {
			t.Fatal("partial assembly budget")
		}
	}
	face, _, _ := roofs[0].OriginalFace()
	for _, limit := range []MixedRoofCollarOptions{{Width: .04, MaxCells: 1}, {Width: .04, MaxPairs: 1}, {Width: 1e-12, MaxDepth: 1}} {
		limit.Roof.Interior.Deflection = .2
		if got, e := n.TessellateRoofFragmentCertified(ctx, face, 1, limit); got != nil || !errors.Is(e, geom.ErrConvergence) {
			t.Fatal("unproved collar publication", e)
		}
	}
	copied := roofs[0].ClearanceCells()
	copied[0].UV[0][0].SetInt64(999)
	if roofs[0].ClearanceCells()[0].UV[0][0].Cmp(big.NewRat(999, 1)) == 0 {
		t.Fatal("mutable collar")
	}
	meshCopy := complete.Mesh()
	meshCopy.Vertices[0].X += 1
	if complete.Mesh().Vertices[0] == meshCopy.Vertices[0] {
		t.Fatal("mutable complete mesh")
	}
	restrictedWitnesses := 0
	for edge := range body.Edges() {
		piece, e := body.SectionPiece(edge)
		if e == nil && piece.Restricted != nil && piece.Index == 1 {
			restrictedWitnesses++
			d, _ := piece.Restricted.Piece(piece.Index)
			if d.Free.Lower.Cmp(big.NewRat(109, 130)) != 0 {
				t.Fatal("changed original material restriction")
			}
		}
	}
	if restrictedWitnesses != 1 {
		t.Fatal("missing original rational restriction", restrictedWitnesses)
	}
	t.Log("complete", len(full.Vertices), len(full.Triangles), "bulk cells", bulk, "collar cells", collars, "collision pairs", work.AssemblyContactPairs, "source squared error", complete.SourceImageSquaredDistance())
}
