package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestMixedOriginalCertifiedBoundary(t *testing.T) {
	ctx := context.Background()
	a, b, p, o := mixedWallFixture(t)
	section, e := SectionSplineShellsMixedAlgebraic(ctx, a, b, p, o)
	if e != nil {
		t.Fatal(e)
	}
	bc := AlgebraicShellBooleanOptions{}
	bc.Imprint.Exact.MaxOperations = 128000000
	bodies, e := section.CombineAlgebraicOperations(ctx, []parametric.BooleanOp{parametric.Intersection}, bc)
	if e != nil {
		t.Fatal(e)
	}
	body := bodies[0]
	options := AlgebraicShellBoundaryOptions{Deflection: .02, MaxOperations: 128000000, AutoSectionProjection: true, PlanarSectionFallback: true}
	network, e := body.TessellateEmbeddedBoundaryCertified(ctx, AlgebraicShellBoundaryEmbeddingOptions{Boundary: options})
	if e != nil {
		t.Fatal(e)
	}
	vertices, edges, uses, ancestry := network.Vertices(), network.Edges(), network.Coedges(), network.Ancestry()
	topo := network.TopologyVertices()
	segments, operations := network.Work()
	embedded, pairs := network.DisplayEmbedded()
	if !embedded || pairs != segments*(segments-1)/2 || len(edges) != len(body.Edges()) || len(uses) != 2*len(edges) {
		t.Fatal("incomplete network")
	}
	counts := make([]int, len(edges))
	balance := make([]int, len(edges))
	for _, u := range uses {
		counts[u.Edge]++
		sign := 1
		if u.Reversed {
			sign = -1
		}
		balance[u.Edge] += sign
		for i, v := range u.Vertices {
			j := i
			if u.Reversed {
				j = len(edges[u.Edge]) - 1 - i
			}
			if v != edges[u.Edge][j] {
				t.Fatal("unshared seam")
			}
		}
	}
	restricted := 0
	for i, path := range edges {
		if counts[i] != 2 || balance[i] != 0 || path[0] != topo[ancestry[i].Start] || path[len(path)-1] != topo[ancestry[i].End] {
			t.Fatal("lost edge incidence")
		}
		cert, e := network.EdgeCertificate(i)
		if e != nil || cert.ErrorUpper > options.Deflection || cert.Segments != len(path)-1 {
			t.Fatal("missing certificate", e)
		}
		if cert.Kind != MixedBoundaryRestrictedCurve {
			continue
		}
		restricted++
		piece, e := body.SectionPiece(i)
		if e != nil {
			t.Fatal(e)
		}
		d, _ := piece.Restricted.Piece(piece.Index)
		if cert.SourceDomain.Lower.Cmp(d.Source.Lower) != 0 || cert.SourceDomain.Upper.Cmp(d.Source.Upper) != 0 {
			t.Fatal("lost original varying-coordinate units")
		}
		if piece.Index == 1 && d.Free.Lower.Cmp(big.NewRat(109, 130)) != 0 {
			t.Fatal("rounded free restriction")
		}
		cells, e := piece.Restricted.ModelCurveCells(ctx, piece.Index, geom.SurfaceCurveAgreementOptions{})
		if e != nil || !cells.Complete {
			t.Fatal(e)
		}
		if limited, e := piece.Restricted.ModelCurveCells(ctx, piece.Index, geom.SurfaceCurveAgreementOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: 1}}); !errors.Is(e, geom.ErrConvergence) || limited.Complete || len(limited.Cells) != 0 {
			t.Fatal("partial model curve", e)
		}
		for end, cell := range []*geom.ExactComposedCurveCell{cells.Cells[0], cells.Cells[len(cells.Cells)-1]} {
			root, e := cell.EndpointRoot(end)
			if e != nil {
				t.Fatal(e)
			}
			interval := root.Interval()
			want := d.Source.Lower
			if end == 1 {
				want = d.Source.Upper
			}
			if interval.Lower.Cmp(want) != 0 || interval.Upper.Cmp(want) != 0 {
				t.Fatal("model parameter changed")
			}
			event, e := cell.BindRoot(ctx, root, geom.ExactCurvePlaneEventOptions{})
			if e != nil {
				t.Fatal(e)
			}
			same, _, e := event.CompareModelPoint(ctx, d.Endpoints[end].ModelEvent, geom.ExactCurvePlaneEventOptions{})
			if e != nil || !same {
				t.Fatal("unmatched chain model", e)
			}
		}
	}
	if restricted != 2 {
		t.Fatal("missing rational certificates", restricted)
	}
	t.Log("vertices", len(vertices), "segments", segments, "operations", operations, "pair checks", pairs)
	vertices[0].X += 1
	edges[0][0] = -1
	uses[0].Vertices[0] = -1
	ancestry[0].Start = -1
	topo[0] = -1
	if network.Vertices()[0] == vertices[0] || network.Edges()[0][0] < 0 || network.Coedges()[0].Vertices[0] < 0 || network.Ancestry()[0].Start < 0 || network.TopologyVertices()[0] < 0 {
		t.Fatal("mutable network")
	}
	for _, bad := range []AlgebraicShellBoundaryOptions{{Deflection: .02, MaxOperations: 1}, {Deflection: .02, MaxSegments: 1}, {Deflection: .02, MaxEdges: 1}} {
		if got, e := body.TessellateBoundaryCertified(ctx, bad); got != nil || !errors.Is(e, geom.ErrConvergence) {
			t.Fatal("partial network", e)
		}
	}
	// Zero pair quota defaults; an explicit insufficient quota cannot claim embedding.
	if _, e := checkBoundaryPolylineEmbedding(ctx, network.Vertices(), network.Edges(), 1); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("pair budget", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if got, e := body.TessellateBoundaryCertified(canceled, options); got != nil || !errors.Is(e, context.Canceled) {
		t.Fatal("canceled network", e)
	}
}
