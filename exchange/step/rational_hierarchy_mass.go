package step

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/geom"
	"reflect"
	"sort"
)

// PiPolynomial is an immutable exact sum of rational coefficients times powers
// of pi. Coefficients returns owned copies, including zeros.
type PiPolynomial struct{ c [5]*big.Rat }

func (p PiPolynomial) Coefficients() (c [5]*big.Rat) {
	for i := range c {
		c[i] = new(big.Rat)
		if p.c[i] != nil {
			c[i].Set(p.c[i])
		}
	}
	return
}

// MassQuotient represents an exact, unreduced quotient. Its denominator is the
// strictly positive material volume at pi, not a formal polynomial unit.
type MassQuotient struct{ Numerator, Denominator PiPolynomial }

// SymbolicMaterialMoments assumes unit density in source length units. Second
// moments are integrals of x_i*x_j; inertia uses the conventional negative products.
type SymbolicMaterialMoments struct {
	v          PiPolynomial
	f          [3]PiPolynomial
	q, inertia [3][3]PiPolynomial
	central    [3][3]MassQuotient
}

func (m SymbolicMaterialMoments) Volume() PiPolynomial                     { return m.v }
func (m SymbolicMaterialMoments) FirstMoments() [3]PiPolynomial            { return m.f }
func (m SymbolicMaterialMoments) SecondMoments() [3][3]PiPolynomial        { return m.q }
func (m SymbolicMaterialMoments) InertiaAboutOrigin() [3][3]PiPolynomial   { return m.inertia }
func (m SymbolicMaterialMoments) InertiaAboutCentroid() [3][3]MassQuotient { return m.central }
func (m SymbolicMaterialMoments) Centroid() (r [3]MassQuotient) {
	for i := range r {
		r[i] = MassQuotient{m.f[i], m.v}
	}
	return
}

// HierarchyMaterialMass retains local definition and original body/face/path IDs.
// Local products have an empty Path; occurrences retain their full source path.
type HierarchyMaterialMass struct {
	ProductID, SourceProductID, SourceBodyID int64
	SourceFaceIDs, Path                      []int64
	Family                                   string
	Moments                                  SymbolicMaterialMoments
}

// RationalHierarchyMass is an occurrence sum, NOT a Boolean union. Overlapping
// instances contribute independently. Products contains each local material once.
type RationalHierarchyMass struct {
	Products, Occurrences []HierarchyMaterialMass
	Sum                   SymbolicMaterialMoments
	LengthUnit            float64
	Triangles             int
	PlanarTests           int
}
type RationalHierarchyMassOptions struct {
	MaxTriangles, MaxCoefficientBits int
	Source                           Limits
}
type massArithmetic struct {
	bits int
	err  error
}

func (a *massArithmetic) check(q *big.Rat) *big.Rat {
	if q.Num().BitLen() > a.bits || q.Denom().BitLen() > a.bits {
		a.err = geom.ErrConvergence
	}
	return q
}
func (a *massArithmetic) add(x, y PiPolynomial) (p PiPolynomial) {
	xc, yc := x.Coefficients(), y.Coefficients()
	for i := range p.c {
		p.c[i] = a.check(new(big.Rat).Add(xc[i], yc[i]))
	}
	return
}
func (a *massArithmetic) scale(x PiPolynomial, q *big.Rat) (p PiPolynomial) {
	xc := x.Coefficients()
	for i := range p.c {
		p.c[i] = a.check(new(big.Rat).Mul(xc[i], q))
	}
	return
}
func (a *massArithmetic) mul(x, y PiPolynomial) (p PiPolynomial) {
	xc, yc := x.Coefficients(), y.Coefficients()
	for i := range p.c {
		p.c[i] = new(big.Rat)
	}
	for i := range xc {
		for j := range yc {
			z := a.check(new(big.Rat).Mul(xc[i], yc[j]))
			if i+j >= 5 {
				if z.Sign() != 0 {
					a.err = geom.ErrConvergence
				}
				continue
			}
			p.c[i+j] = a.check(new(big.Rat).Add(p.c[i+j], z))
		}
	}
	return
}
func (a *massArithmetic) scalar(q *big.Rat, d int) (p PiPolynomial) {
	p.c[d] = a.check(new(big.Rat).Set(q))
	return
}
func (a *massArithmetic) finish(m SymbolicMaterialMoments) SymbolicMaterialMoments {
	var tr, ff PiPolynomial
	for i := 0; i < 3; i++ {
		tr = a.add(tr, m.q[i][i])
		ff = a.add(ff, a.mul(m.f[i], m.f[i]))
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			m.inertia[i][j] = a.scale(m.q[i][j], big.NewRat(-1, 1))
			shift := a.scale(a.mul(m.f[i], m.f[j]), big.NewRat(-1, 1))
			if i == j {
				m.inertia[i][j] = a.add(m.inertia[i][j], tr)
				shift = a.add(shift, ff)
			}
			m.central[i][j] = MassQuotient{a.add(a.mul(m.inertia[i][j], m.v), a.scale(shift, big.NewRat(-1, 1))), m.v}
		}
	}
	return m
}
func (a *massArithmetic) transform(m SymbolicMaterialMoments, r [3][4]*big.Rat) (n SymbolicMaterialMoments) {
	n.v = m.v
	var f [3]PiPolynomial
	for i := 0; i < 3; i++ {
		for k := 0; k < 3; k++ {
			f[i] = a.add(f[i], a.scale(m.f[k], r[i][k]))
		}
		n.f[i] = a.add(f[i], a.scale(m.v, r[i][3]))
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			for k := 0; k < 3; k++ {
				for l := 0; l < 3; l++ {
					n.q[i][j] = a.add(n.q[i][j], a.scale(m.q[k][l], a.check(new(big.Rat).Mul(r[i][k], r[j][l]))))
				}
			}
			n.q[i][j] = a.add(n.q[i][j], a.scale(f[j], r[i][3]))
			n.q[i][j] = a.add(n.q[i][j], a.scale(f[i], r[j][3]))
			n.q[i][j] = a.add(n.q[i][j], a.scale(m.v, a.check(new(big.Rat).Mul(r[i][3], r[j][3]))))
		}
	}
	return n
}

// MeasureRationalSphericalHierarchy re-proves the complete original graph and
// caller claims before integrating local products and exact proper occurrences.
func MeasureRationalSphericalHierarchy(ctx context.Context, m RationalSphericalHierarchy, o RationalHierarchyMassOptions) (RationalHierarchyMass, error) {
	return measureRationalHierarchy(ctx, m, o, false)
}
func MeasureOCCTRationalSphericalHierarchy(ctx context.Context, m OCCTRationalSphericalHierarchy, o RationalHierarchyMassOptions) (RationalHierarchyMass, error) {
	return measureRationalHierarchy(ctx, RationalSphericalHierarchy(m), o, true)
}
func measureRationalHierarchy(ctx context.Context, m RationalSphericalHierarchy, o RationalHierarchyMassOptions, vendor bool, general ...int) (RationalHierarchyMass, error) {
	bad := func(e error) (RationalHierarchyMass, error) { return RationalHierarchyMass{}, e }
	if ctx == nil {
		return bad(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return bad(e)
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 65536
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 4096
	}
	if o.MaxTriangles < 1 || o.MaxTriangles > 1048576 || o.MaxCoefficientBits < 256 || o.MaxCoefficientBits > 8192 {
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
	a := massArithmetic{bits: o.MaxCoefficientBits}
	out := RationalHierarchyMass{LengthUnit: proof.LengthUnit}
	ids := make([]int64, 0, len(proof.Products))
	for id := range proof.Products {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	locals := map[int64]HierarchyMaterialMass{}
	for _, id := range ids {
		if e = ctx.Err(); e != nil {
			return bad(e)
		}
		p := proof.Products[id]
		v := HierarchyMaterialMass{ProductID: id, SourceProductID: p.SourceProductID, SourceBodyID: p.SourceBodyID}
		body, e := simple(proof.Source, p.SourceBodyID, "MANIFOLD_SOLID_BREP", 2)
		if e != nil {
			return bad(e)
		}
		shell, e := simple(proof.Source, body.Args[1].Ref, "CLOSED_SHELL", 2)
		if e != nil {
			return bad(e)
		}
		v.SourceFaceIDs, e = listRefs(shell.Args[1], 1)
		if e != nil {
			return bad(e)
		}
		var mass SymbolicMaterialMoments
		if p.Planar != nil {
			v.Family = "planar"
			if len(general) != 0 {
				mass, e = integrateGeneralHierarchyPlanar(ctx, p, v.SourceFaceIDs, &a, &out, o.MaxTriangles, general[0])
				if e != nil {
					return bad(e)
				}
			} else {
				mesh := p.Planar.Solid.Tessellate()
				out.Triangles += len(mesh.Triangles)
				if out.Triangles > o.MaxTriangles {
					return bad(geom.ErrConvergence)
				}
				for _, t := range mesh.Triangles {
					if e = ctx.Err(); e != nil {
						return bad(e)
					}
					x := [3][3]*big.Rat{mappedRatVec(mesh.Vertices[t[0]]), mappedRatVec(mesh.Vertices[t[1]]), mappedRatVec(mesh.Vertices[t[2]])}
					det := new(big.Rat)
					for i := 0; i < 3; i++ {
						z := new(big.Rat).Sub(new(big.Rat).Mul(x[1][(i+1)%3], x[2][(i+2)%3]), new(big.Rat).Mul(x[1][(i+2)%3], x[2][(i+1)%3]))
						det.Add(det, a.check(new(big.Rat).Mul(x[0][i], z)))
					}
					vol := a.scalar(new(big.Rat).Quo(det, big.NewRat(6, 1)), 0)
					mass.v = a.add(mass.v, vol)
					var s [3]*big.Rat
					for i := 0; i < 3; i++ {
						s[i] = a.check(new(big.Rat).Add(new(big.Rat).Add(x[0][i], x[1][i]), x[2][i]))
						mass.f[i] = a.add(mass.f[i], a.scale(vol, new(big.Rat).Quo(s[i], big.NewRat(4, 1))))
					}
					for i := 0; i < 3; i++ {
						for j := 0; j < 3; j++ {
							z := a.check(new(big.Rat).Mul(s[i], s[j]))
							for k := 0; k < 3; k++ {
								z.Add(z, a.check(new(big.Rat).Mul(x[k][i], x[k][j])))
							}
							mass.q[i][j] = a.add(mass.q[i][j], a.scale(vol, a.check(new(big.Rat).Quo(z, big.NewRat(20, 1)))))
						}
					}
				}
			}
			if mass.v.Coefficients()[0].Sign() <= 0 {
				return bad(geom.ErrInvalid)
			}
		} else {
			var frame geom.Frame
			var r, h, R *big.Rat
			degree := 1
			var volume, radial, axial *big.Rat
			switch {
			case p.Cylinder != nil:
				v.Family = "cylinder"
				frame = p.Cylinder.Frame
				r = new(big.Rat).SetFloat64(p.Cylinder.Radius)
				h = new(big.Rat).SetFloat64(p.Cylinder.Height)
				r2 := new(big.Rat).Mul(r, r)
				volume = new(big.Rat).Mul(r2, h)
				radial = new(big.Rat).Quo(r2, big.NewRat(4, 1))
				axial = new(big.Rat).Quo(new(big.Rat).Mul(h, h), big.NewRat(12, 1))
			case p.Sphere != nil:
				v.Family = "sphere"
				frame = p.Sphere.Frame
				r = new(big.Rat).SetFloat64(p.Sphere.Radius)
				r2 := new(big.Rat).Mul(r, r)
				volume = new(big.Rat).Mul(new(big.Rat).Mul(r2, r), big.NewRat(4, 3))
				radial = new(big.Rat).Quo(r2, big.NewRat(5, 1))
				axial = new(big.Rat).Set(radial)
			case p.Torus != nil:
				v.Family = "torus"
				degree = 2
				frame = p.Torus.Frame
				r = new(big.Rat).SetFloat64(p.Torus.MinorRadius)
				R = new(big.Rat).SetFloat64(p.Torus.MajorRadius)
				r2 := new(big.Rat).Mul(r, r)
				volume = new(big.Rat).Mul(new(big.Rat).Mul(R, r2), big.NewRat(2, 1))
				radial = new(big.Rat).Add(new(big.Rat).Mul(new(big.Rat).Mul(R, R), big.NewRat(1, 2)), new(big.Rat).Mul(r2, big.NewRat(3, 8)))
				axial = new(big.Rat).Mul(r2, big.NewRat(1, 4))
			default:
				return bad(geom.ErrInvalid)
			}
			mass.v = a.scalar(volume, degree)
			mass.q[0][0] = a.scale(mass.v, radial)
			mass.q[1][1] = mass.q[0][0]
			mass.q[2][2] = a.scale(mass.v, axial)
			axes := [3][3]*big.Rat{mappedRatVec(frame.XAxis()), mappedRatVec(frame.YAxis()), mappedRatVec(frame.ZAxis())}
			center := mappedRatVec(frame.Origin())
			var mat [3][4]*big.Rat
			for i := 0; i < 3; i++ {
				for j := 0; j < 3; j++ {
					mat[i][j] = axes[j][i]
				}
				mat[i][3] = center[i]
				if h != nil {
					mat[i][3] = a.check(new(big.Rat).Add(center[i], new(big.Rat).Mul(axes[2][i], new(big.Rat).Quo(h, big.NewRat(2, 1)))))
				}
			}
			mass = a.transform(mass, mat)
		}
		v.Moments = a.finish(mass)
		if a.err != nil {
			return bad(a.err)
		}
		locals[id] = v
		out.Products = append(out.Products, v)
	}
	for _, u := range proof.Instances {
		if e = ctx.Err(); e != nil {
			return bad(e)
		}
		v := locals[u.ProductID]
		v.Path = append([]int64(nil), u.Path...)
		v.SourceFaceIDs = append([]int64(nil), v.SourceFaceIDs...)
		v.Moments = a.finish(a.transform(v.Moments, u.Matrix()))
		out.Occurrences = append(out.Occurrences, v)
		out.Sum.v = a.add(out.Sum.v, v.Moments.v)
		for i := 0; i < 3; i++ {
			out.Sum.f[i] = a.add(out.Sum.f[i], v.Moments.f[i])
			for j := 0; j < 3; j++ {
				out.Sum.q[i][j] = a.add(out.Sum.q[i][j], v.Moments.q[i][j])
			}
		}
		if a.err != nil {
			return bad(a.err)
		}
	}
	out.Sum = a.finish(out.Sum)
	if a.err != nil {
		return bad(a.err)
	}
	return out, nil
}
