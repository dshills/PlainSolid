package step

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/geom"
	"reflect"
	"sort"
)

// HierarchyFaceArea retains a proved original face/support/bound association.
// SignedLoopAreas is outer-positive and hole-negative, in source length squared;
// its strings are exact rationals. Analytic faces use Formula instead.
type HierarchyFaceArea struct {
	SourceFaceID, SourceSurfaceID int64
	SourceBoundIDs                []int64
	SignedLoopAreas               []string
	Formula                       string
	Area                          PiPolynomial
}

// HierarchyMaterialArea measures a local definition or one proper rigid use.
// A local product has no Path. Occurrences retain the full original NAUO path.
type HierarchyMaterialArea struct {
	ProductID, SourceProductID, SourceBodyID int64
	Path                                     []int64
	Family                                   string
	Faces                                    []HierarchyFaceArea
	Area                                     PiPolynomial
}

// RationalHierarchyArea sums material occurrence boundaries, not a Boolean-union
// boundary. Shared/contacting or overlapping uses are counted independently.
// Areas are in the common source unit squared; LengthUnit is metres per unit.
type RationalHierarchyArea struct {
	Products, Occurrences  []HierarchyMaterialArea
	Sum                    PiPolynomial
	LengthUnit             float64
	PlanarTests, LoopEdges int
}
type RationalHierarchyAreaOptions struct {
	MaxPlanarTests, MaxLoopEdges, MaxCoefficientBits int
	Source                                           Limits
}

// MeasureRationalHierarchyBoundaryArea re-proves the complete source graph
// before measuring each original face and its proper rational occurrences.
func MeasureRationalHierarchyBoundaryArea(ctx context.Context, m RationalSphericalHierarchy, o RationalHierarchyAreaOptions) (RationalHierarchyArea, error) {
	return measureHierarchyArea(ctx, m, o, false)
}

// MeasureOCCTRationalHierarchyBoundaryArea selects the explicit OCCT frame rule.
func MeasureOCCTRationalHierarchyBoundaryArea(ctx context.Context, m OCCTRationalSphericalHierarchy, o RationalHierarchyAreaOptions) (RationalHierarchyArea, error) {
	return measureHierarchyArea(ctx, RationalSphericalHierarchy(m), o, true)
}

// BoundaryArea includes the original affine pcurve face ancestry in its result.
func (m AffinePCurveHierarchy) BoundaryArea(ctx context.Context, o RationalHierarchyAreaOptions) (RationalHierarchyArea, error) {
	return measureHierarchyArea(ctx, m.model, o, m.vendor)
}

func measureHierarchyArea(ctx context.Context, m RationalSphericalHierarchy, o RationalHierarchyAreaOptions, vendor bool) (RationalHierarchyArea, error) {
	bad := func(e error) (RationalHierarchyArea, error) { return RationalHierarchyArea{}, e }
	if ctx == nil {
		return bad(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return bad(e)
	}
	if o.MaxPlanarTests == 0 {
		o.MaxPlanarTests = 65536
	}
	if o.MaxLoopEdges == 0 {
		o.MaxLoopEdges = 65536
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 4096
	}
	if o.MaxPlanarTests < 1 || o.MaxPlanarTests > 1048576 || o.MaxLoopEdges < 1 || o.MaxLoopEdges > 1048576 || o.MaxCoefficientBits < 256 || o.MaxCoefficientBits > 8192 {
		return bad(geom.ErrInvalid)
	}
	actual, c, e := rationalSphericalHierarchyEnvelopeFor(ctx, m, vendor)
	if e != nil {
		return bad(e)
	}
	proof, e := readRationalSphericalHierarchy(ctx, bytes.NewBufferString(actual.Source), o.Source, actual.LocalProof.Proof.Tolerance, vendor)
	if e != nil {
		return bad(e)
	}
	expected, d, e := rationalSphericalHierarchyEnvelopeFor(ctx, proof, vendor)
	if e != nil {
		return bad(e)
	}
	if !reflect.DeepEqual(actual, expected) || !reflect.DeepEqual(c, d) {
		return bad(geom.ErrInvalid)
	}
	out := RationalHierarchyArea{LengthUnit: proof.LengthUnit}
	a := massArithmetic{bits: o.MaxCoefficientBits}
	pa := hierarchyPointArithmetic{bits: o.MaxCoefficientBits}
	tick := func() error {
		out.PlanarTests++
		if out.PlanarTests > o.MaxPlanarTests {
			return geom.ErrConvergence
		}
		return ctx.Err()
	}
	ids := make([]int64, 0, len(proof.Products))
	for id := range proof.Products {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	locals := map[int64]HierarchyMaterialArea{}
	for _, id := range ids {
		if e = ctx.Err(); e != nil {
			return bad(e)
		}
		p := proof.Products[id]
		body, e := simple(proof.Source, p.SourceBodyID, "MANIFOLD_SOLID_BREP", 2)
		if e != nil {
			return bad(e)
		}
		shell, e := simple(proof.Source, body.Args[1].Ref, "CLOSED_SHELL", 2)
		if e != nil {
			return bad(e)
		}
		faces, e := listRefs(shell.Args[1], 1)
		if e != nil {
			return bad(e)
		}
		v := HierarchyMaterialArea{ProductID: id, SourceProductID: p.SourceProductID, SourceBodyID: p.SourceBodyID}
		for _, fid := range faces {
			fr, e := simple(proof.Source, fid, "ADVANCED_FACE", 4)
			if e != nil {
				return bad(e)
			}
			bounds, e := listRefs(fr.Args[1], 1)
			if e != nil {
				return bad(e)
			}
			surface, e := ref(fr.Args[2])
			if e != nil {
				return bad(e)
			}
			v.Faces = append(v.Faces, HierarchyFaceArea{SourceFaceID: fid, SourceSurfaceID: surface, SourceBoundIDs: append([]int64(nil), bounds...)})
		}
		switch {
		case p.Planar != nil:
			v.Family = "planar"
			g, e := prepareHierarchyGeneralPlanar(ctx, p, faces, &pa, tick)
			if e != nil {
				return bad(e)
			}
			for fi, face := range g.faces {
				v.Faces[fi].SourceBoundIDs = append([]int64(nil), face.boundIDs...)
				// The source gate proves a signed-cardinal unit normal. Proper rational
				// world rotations preserve its norm exactly; no square root is rounded.
				n := face.plane.n
				norm2 := pa.planarDot(n, n)
				if norm2.Cmp(big.NewRat(1, 1)) != 0 || new(big.Rat).Abs(n[face.drop]).Cmp(big.NewRat(1, 1)) != 0 {
					return bad(geom.ErrDomain)
				}
				i, j := (face.drop+1)%3, (face.drop+2)%3
				sum := new(big.Rat)
				for _, loop := range face.loops {
					twice := new(big.Rat)
					for k, vi := range loop {
						out.LoopEdges++
						if out.LoopEdges > o.MaxLoopEdges {
							return bad(geom.ErrConvergence)
						}
						if e = ctx.Err(); e != nil {
							return bad(e)
						}
						x, y := g.base.vertices[vi], g.base.vertices[loop[(k+1)%len(loop)]]
						twice = pa.add(twice, pa.sub(pa.mul(x[i], y[j]), pa.mul(x[j], y[i])))
					}
					area := pa.mul(twice, big.NewRat(int64(n[face.drop].Sign()), 2))
					v.Faces[fi].SignedLoopAreas = append(v.Faces[fi].SignedLoopAreas, area.RatString())
					sum = pa.add(sum, area)
				}
				if pa.err != nil {
					return bad(pa.err)
				}
				if sum.Sign() <= 0 {
					return bad(geom.ErrInvalid)
				}
				v.Faces[fi].Formula = "oriented-planar-loops"
				v.Faces[fi].Area = a.scalar(sum, 0)
			}
		case p.Cylinder != nil:
			v.Family = "cylinder"
			r, h := new(big.Rat).SetFloat64(p.Cylinder.Radius), new(big.Rat).SetFloat64(p.Cylinder.Height)
			walls, caps := 0, 0
			for i, f := range v.Faces {
				en := proof.Source.Entities[f.SourceSurfaceID]
				if len(en.Records) != 1 {
					return bad(geom.ErrInvalid)
				}
				switch en.Records[0].Type {
				case "PLANE":
					caps++
					v.Faces[i].Formula = "pi*r^2"
					v.Faces[i].Area = a.scalar(a.check(new(big.Rat).Mul(r, r)), 1)
				case "CYLINDRICAL_SURFACE":
					walls++
					v.Faces[i].Formula = "2*pi*r*h/cells"
					q := a.check(new(big.Rat).Mul(r, h))
					q = a.check(new(big.Rat).Mul(q, big.NewRat(2, int64(p.Cylinder.Cells))))
					v.Faces[i].Area = a.scalar(q, 1)
				default:
					return bad(geom.ErrInvalid)
				}
			}
			if caps != 2 || walls != p.Cylinder.Cells || (walls != 2 && walls != 4) {
				return bad(geom.ErrInvalid)
			}
		case p.Sphere != nil:
			v.Family = "sphere"
			if len(v.Faces) != 1 || v.Faces[0].SourceFaceID != p.Sphere.SourceFaceID {
				return bad(geom.ErrInvalid)
			}
			r := new(big.Rat).SetFloat64(p.Sphere.Radius)
			v.Faces[0].Formula = "4*pi*r^2"
			v.Faces[0].Area = a.scalar(a.check(new(big.Rat).Mul(a.check(new(big.Rat).Mul(r, r)), big.NewRat(4, 1))), 1)
		case p.Torus != nil:
			v.Family = "torus"
			if len(v.Faces) != 1 || v.Faces[0].SourceFaceID != p.Torus.SourceFaceID {
				return bad(geom.ErrInvalid)
			}
			r, R := new(big.Rat).SetFloat64(p.Torus.MinorRadius), new(big.Rat).SetFloat64(p.Torus.MajorRadius)
			v.Faces[0].Formula = "4*pi^2*R*r"
			v.Faces[0].Area = a.scalar(a.check(new(big.Rat).Mul(a.check(new(big.Rat).Mul(r, R)), big.NewRat(4, 1))), 2)
		default:
			return bad(geom.ErrDomain)
		}
		for _, f := range v.Faces {
			v.Area = a.add(v.Area, f.Area)
		}
		if a.err != nil {
			return bad(a.err)
		}
		locals[id] = v
		out.Products = append(out.Products, copyHierarchyMaterialArea(v))
	}
	for _, u := range proof.Instances {
		if e = ctx.Err(); e != nil {
			return bad(e)
		}
		v := copyHierarchyMaterialArea(locals[u.ProductID])
		v.Path = append([]int64(nil), u.Path...)
		out.Occurrences = append(out.Occurrences, v)
		out.Sum = a.add(out.Sum, v.Area)
		if a.err != nil {
			return bad(a.err)
		}
	}
	return out, nil
}
func copyHierarchyMaterialArea(v HierarchyMaterialArea) HierarchyMaterialArea {
	v.Path = append([]int64(nil), v.Path...)
	v.Faces = append([]HierarchyFaceArea(nil), v.Faces...)
	for i := range v.Faces {
		v.Faces[i].SourceBoundIDs = append([]int64(nil), v.Faces[i].SourceBoundIDs...)
		v.Faces[i].SignedLoopAreas = append([]string(nil), v.Faces[i].SignedLoopAreas...)
	}
	return v
}
