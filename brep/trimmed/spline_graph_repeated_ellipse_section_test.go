package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestSectionRepeatedRationalEllipseSplineGraphs(t *testing.T) {
	ctx := context.Background()
	// In normalized original UV, B has height u*v and A has height u*v+q².
	// The mixed term gives a rotated ellipse and its common model loop is
	// curved in all three coordinates. Different original knots cross it.
	h := geom.GraphQuadraticEllipseHint{Coefficients: [6]*big.Rat{
		big.NewRat(12, 1), big.NewRat(-24, 1), big.NewRat(16, 1), big.NewRat(-28, 1), big.NewRat(20, 1), big.NewRat(16, 1)}}
	indices := [6][2]int{{0, 0}, {1, 0}, {2, 0}, {0, 1}, {0, 2}, {1, 1}}
	var supports [2]*geom.SurfacePatch
	for owner := range supports {
		var net [][4]*big.Rat
		for i := range 5 {
			for j := range 5 {
				x, y := big.NewRat(int64(i), 4), big.NewRat(int64(j), 4)
				z := new(big.Rat).Mul(x, y)
				if owner == 0 {
					for a, ij := range indices {
						for b, kl := range indices {
							u, v := int64(ij[0]+kl[0]), int64(ij[1]+kl[1])
							if u > int64(i) || v > int64(j) {
								continue
							}
							weight := new(big.Rat).SetFrac(new(big.Int).Mul(new(big.Int).Binomial(int64(i), u), new(big.Int).Binomial(int64(j), v)),
								new(big.Int).Mul(new(big.Int).Binomial(4, u), new(big.Int).Binomial(4, v)))
							z.Add(z, new(big.Rat).Mul(weight, new(big.Rat).Mul(h.Coefficients[a], h.Coefficients[b])))
						}
					}
				}
				net = append(net, [4]*big.Rat{x, y, z, big.NewRat(1, 1)})
			}
		}
		p, e := geom.NewExactHomogeneousBezierSurface(ctx, 4, 4, net, geom.SurfacePatchOptions{})
		if e == nil {
			if owner == 0 {
				p, e = p.InsertUKnot(.5, 1, geom.SurfacePatchOptions{})
			} else {
				p, e = p.InsertVKnot(.375, 1, geom.SurfacePatchOptions{})
			}
		}
		if e != nil {
			t.Fatal(e)
		}
		supports[owner] = p
	}
	discovered, e := geom.DiscoverSplineGraphRepeatedQuadraticEllipse(ctx, supports[0], supports[1], geom.GraphPointContactOptions{})
	if e != nil || !discovered.Proven || discovered.InsideSide != 1 || len(discovered.CertifiedCells) != 4 ||
		discovered.Ellipse.Coefficients[5].Cmp(discovered.Ellipse.Coefficients[2]) != 0 {
		t.Fatal("rotated factor discovered and re-proved across both original knot sets", discovered, e)
	}
	budget := geom.GraphPointContactOptions{MaxOperations: discovered.ExactOperations - 1}
	if r, e := geom.DiscoverSplineGraphRepeatedQuadraticEllipse(ctx, supports[0], supports[1], budget); e == nil || r.Proven {
		t.Fatal("one-short work budget must not produce a complete certificate", e)
	}
	point := [2]*big.Rat{big.NewRat(3, 4), big.NewRat(1, 2)}
	section, e := SectionRepeatedRationalEllipseSplineGraphs(ctx, supports[0], supports[1], h, point,
		geom.GraphPointContactOptions{}, geom.SurfaceCurveAgreementOptions{})
	if e != nil || !section.UV.Complete || section.UV.Certificate.InsideSide != 1 || section.UV.Certificate.OutsideSide != 1 {
		t.Fatal("exact closed double contact", e)
	}
	for _, row := range section.Model {
		for i, arc := range row {
			if !arc.Complete || len(arc.Cells) == 0 {
				t.Fatal("original knot cells were not retained", i)
			}
			p, e := arc.Cells[0].EvaluateRational(ctx, new(big.Rat), geom.ExactCurvePlaneEventOptions{})
			u, v := section.UV.Junctions[i][0], section.UV.Junctions[i][1]
			if e != nil || p.Point[0].Cmp(u) != 0 || p.Point[1].Cmp(v) != 0 || p.Point[2].Cmp(new(big.Rat).Mul(u, v)) != 0 {
				t.Fatal("original model-space junction", i, p, e)
			}
		}
	}
	mesh, e := section.TessellateCertified(ctx, geom.ComposedCurvePolylineOptions{Deflection: .01})
	if e != nil || len(mesh.Segments) < 8 || mesh.MaximumError > .01 {
		t.Fatal("tangent section display", mesh, e)
	}
	degree := make([]int, len(mesh.Vertices))
	for _, s := range mesh.Segments {
		for _, v := range s.Vertices {
			degree[v]++
		}
	}
	for _, d := range degree {
		if d != 2 {
			t.Fatal("tangent loop vertices must be shared", degree)
		}
	}
	point[0] = big.NewRat(1, 2)
	if r, e := SectionRepeatedRationalEllipseSplineGraphs(ctx, supports[0], supports[1], h, point,
		geom.GraphPointContactOptions{}, geom.SurfaceCurveAgreementOptions{}); !errors.Is(e, geom.ErrDomain) || r.UV.Complete {
		t.Fatal("off-conic point was accepted", e)
	}
}
