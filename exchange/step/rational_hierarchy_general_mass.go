package step

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// GeneralRationalHierarchyMassOptions bounds both the exact source-loop binding
// work and signed boundary integration. MaxTriangles counts signed fan terms,
// not a geometric triangulation. Limits are cumulative across local products.
type GeneralRationalHierarchyMassOptions struct {
	RationalHierarchyMassOptions
	MaxPlanarTests int
}

func generalHierarchyMass(ctx context.Context, m RationalSphericalHierarchy, o GeneralRationalHierarchyMassOptions, vendor bool) (RationalHierarchyMass, error) {
	if o.MaxPlanarTests == 0 {
		o.MaxPlanarTests = 65536
	}
	if o.MaxPlanarTests < 1 || o.MaxPlanarTests > 1048576 {
		return RationalHierarchyMass{}, geom.ErrInvalid
	}
	return measureRationalHierarchy(ctx, m, o.RationalHierarchyMassOptions, vendor, o.MaxPlanarTests)
}

// MeasureGeneralRationalSphericalHierarchy integrates authenticated concave and
// holed planar faces by oriented rational boundary chains. It sums occurrences,
// not their union; analytic products retain their existing symbolic formulas.
func MeasureGeneralRationalSphericalHierarchy(ctx context.Context, m RationalSphericalHierarchy, o GeneralRationalHierarchyMassOptions) (RationalHierarchyMass, error) {
	return generalHierarchyMass(ctx, m, o, false)
}

// MeasureGeneralOCCTRationalSphericalHierarchy uses the explicit OCCT frame rule.
func MeasureGeneralOCCTRationalSphericalHierarchy(ctx context.Context, m OCCTRationalSphericalHierarchy, o GeneralRationalHierarchyMassOptions) (RationalHierarchyMass, error) {
	return generalHierarchyMass(ctx, RationalSphericalHierarchy(m), o, true)
}

// Measure computes exact moments of the authenticated affine-pcurve hierarchy.
// Its private proof view preserves original face and occurrence ancestry.
func (m AffinePCurveHierarchy) Measure(ctx context.Context, o GeneralRationalHierarchyMassOptions) (RationalHierarchyMass, error) {
	return generalHierarchyMass(ctx, m.model, o, m.vendor)
}

func integrateGeneralHierarchyPlanar(ctx context.Context, p IndependentExactMaterial, faces []int64, a *massArithmetic, out *RationalHierarchyMass, maxTriangles, maxTests int) (SymbolicMaterialMoments, error) {
	var mass SymbolicMaterialMoments
	tick := func() error {
		out.PlanarTests++
		if out.PlanarTests > maxTests {
			return geom.ErrConvergence
		}
		return ctx.Err()
	}
	pa := hierarchyPointArithmetic{bits: a.bits}
	proof, e := prepareHierarchyGeneralPlanar(ctx, p, faces, &pa, tick)
	if e != nil {
		return mass, e
	}
	// A planar directed polygon equals its signed fan as a boundary chain.
	// This remains valid for concavity and holes: overlaps cancel algebraically.
	// Global embedded/outward material was already re-proved by the source reader.
	for _, face := range proof.faces {
		for _, loop := range face.loops {
			for k := 1; k+1 < len(loop); k++ {
				out.Triangles++
				if out.Triangles > maxTriangles {
					return SymbolicMaterialMoments{}, geom.ErrConvergence
				}
				if e = ctx.Err(); e != nil {
					return SymbolicMaterialMoments{}, e
				}
				x := [3][3]*big.Rat{proof.base.vertices[loop[0]], proof.base.vertices[loop[k]], proof.base.vertices[loop[k+1]]}
				integrateSignedHierarchyTetra(&mass, x, a)
				if a.err != nil {
					return SymbolicMaterialMoments{}, a.err
				}
			}
		}
	}
	if mass.v.Coefficients()[0].Sign() <= 0 {
		return SymbolicMaterialMoments{}, geom.ErrInvalid
	}
	return mass, nil
}
func integrateSignedHierarchyTetra(mass *SymbolicMaterialMoments, x [3][3]*big.Rat, a *massArithmetic) {
	det := new(big.Rat)
	for i := 0; i < 3; i++ {
		z := a.check(new(big.Rat).Sub(a.check(new(big.Rat).Mul(x[1][(i+1)%3], x[2][(i+2)%3])), a.check(new(big.Rat).Mul(x[1][(i+2)%3], x[2][(i+1)%3]))))
		det = a.check(new(big.Rat).Add(det, a.check(new(big.Rat).Mul(x[0][i], z))))
	}
	vol := a.scalar(a.check(new(big.Rat).Quo(det, big.NewRat(6, 1))), 0)
	mass.v = a.add(mass.v, vol)
	var sum [3]*big.Rat
	for i := 0; i < 3; i++ {
		sum[i] = a.check(new(big.Rat).Add(a.check(new(big.Rat).Add(x[0][i], x[1][i])), x[2][i]))
		mass.f[i] = a.add(mass.f[i], a.scale(vol, a.check(new(big.Rat).Quo(sum[i], big.NewRat(4, 1)))))
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			z := a.check(new(big.Rat).Mul(sum[i], sum[j]))
			for k := 0; k < 3; k++ {
				z = a.check(new(big.Rat).Add(z, a.check(new(big.Rat).Mul(x[k][i], x[k][j]))))
			}
			mass.q[i][j] = a.add(mass.q[i][j], a.scale(vol, a.check(new(big.Rat).Quo(z, big.NewRat(20, 1)))))
		}
	}
}
