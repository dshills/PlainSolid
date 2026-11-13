package trimmed

import (
	"context"
	"math"
	"math/big"

	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// CertifiedRevolvedCircleMesh certifies the closed display against the ideal
// represented-affine torus or elbow. The native analytic Solid independently
// retains its numerical cap sewing tolerance. OriginalFace maps display
// triangles to its torus support and, for an elbow, its two planar caps.
type CertifiedRevolvedCircleMesh struct {
	Solid                                             Solid
	Mesh                                              mesh.Mesh
	OriginalFace                                      []int
	IdealBoundarySquaredDistance, SignedDisplayVolume *big.Rat
	FrameDeterminant, DisplayVertexSquaredError       *big.Rat
	Collision                                         collision.Report
	AngularSegments, MeridianSegments, TrigTerms      int
}

type torusSample struct{ sn, cs, err *big.Rat }

// RevolveCircleCertified supports signed partial turns up to six radians and
// full turns specified by either sign of 2*math.Pi. A full turn uses true
// mathematical 2π with shared index seams. Exact trig enclosures and an
// analytic second derivative bound certify both directed display distances.
// The mesh embedding is checked independently. MaxSegments bounds each
// angular direction and MaxTrigTerms bounds both exact sample grids.
func RevolveCircleCertified(ctx context.Context, frame geom.Frame, major, axial, minor, angle float64, o CertifiedRevolvePolygonOptions) (CertifiedRevolvedCircleMesh, error) {
	var empty CertifiedRevolvedCircleMesh
	if ctx == nil || !finite(o.Deflection) || o.Deflection <= 0 || !finite(angle) || angle == 0 {
		return empty, geom.ErrInvalid
	}
	full := math.Abs(angle) == 2*math.Pi
	if !full && math.Abs(angle) > 6 {
		return empty, geom.ErrInvalid
	}
	if o.MaxSegments == 0 {
		o.MaxSegments = 512
	}
	if o.MaxVertices == 0 {
		o.MaxVertices = 65536
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 262144
	}
	if o.MaxTrigTerms == 0 {
		o.MaxTrigTerms = 262144
	}
	if o.MaxBits == 0 {
		o.MaxBits = 16384
	}
	if o.MaxSegments < 1 || o.MaxSegments > 4096 || o.MaxVertices < 4 || o.MaxVertices > 1000000 ||
		o.MaxTriangles < 4 || o.MaxTriangles > 2000000 || o.MaxTrigTerms < 1 || o.MaxTrigTerms > 2000000 ||
		o.MaxBits < 64 || o.MaxBits > 65536 {
		return empty, geom.ErrInvalid
	}
	if !frame.Origin().IsFinite() || !frame.XAxis().IsFinite() || !frame.YAxis().IsFinite() || !frame.ZAxis().IsFinite() {
		return empty, geom.ErrInvalid
	}
	m := [3][3]*big.Rat{exactBoundPoint(frame.XAxis()), exactBoundPoint(frame.YAxis()), exactBoundPoint(frame.ZAxis())}
	origin := exactBoundPoint(frame.Origin())
	det := new(big.Rat)
	// A separate exact determinant proves the ideal affine map is
	// embedded. Its squared Euclidean operator norm is bounded by the
	// maximum absolute row sum of the Gram matrix.
	norm2 := new(big.Rat)
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		det.Add(det, new(big.Rat).Mul(m[0][i], new(big.Rat).Sub(new(big.Rat).Mul(m[1][j], m[2][k]), new(big.Rat).Mul(m[1][k], m[2][j]))))
		row := new(big.Rat)
		for j := 0; j < 3; j++ {
			dot := new(big.Rat)
			for k := 0; k < 3; k++ {
				dot.Add(dot, new(big.Rat).Mul(m[i][k], m[j][k]))
			}
			row.Add(row, dot.Abs(dot))
		}
		if row.Cmp(norm2) > 0 {
			norm2.Set(row)
		}
	}
	if det.Sign() <= 0 {
		return empty, geom.ErrDegenerate
	}
	source, e := RevolveCircle(ctx, frame, major, axial, minor, angle)
	if e != nil {
		return empty, e
	}
	r, radius, z := ratInset(minor), ratInset(major), ratInset(axial)
	rmax := new(big.Rat).Add(radius, r)
	tol := ratInset(o.Deflection)
	tol2 := new(big.Rat).Mul(tol, tol)
	lo, hi := ratInset(min(0, angle)), ratInset(max(0, angle))
	uUpper := new(big.Rat).Sub(hi, lo)
	vUpper := big.NewRat(44, 7) // 2π < 44/7
	if full {
		uUpper = vUpper
	}
	du, dv := new(big.Rat), new(big.Rat)
	nu, nv := 1, 4
	if full {
		nu = 4
	}
	var curvatureSquared *big.Rat
	for nu <= o.MaxSegments && nv <= o.MaxSegments {
		du.Quo(uUpper, big.NewRat(int64(nu), 1))
		dv.Quo(vUpper, big.NewRat(int64(nv), 1))
		// Along a UV line, ||D²F|| is bounded by
		// (R+r)du² + 2r du dv + r dv². Each display triangle is an
		// interpolant with barycentric parameters; first derivatives
		// cancel. The variance of each scalar coordinate is <= range²/4,
		// and the mixed term follows Cauchy-Schwarz.
		uTerm := new(big.Rat).Mul(rmax, new(big.Rat).Mul(du, du))
		vTerm := new(big.Rat).Mul(r, new(big.Rat).Mul(dv, dv))
		mixed := new(big.Rat).Mul(new(big.Rat).Mul(big.NewRat(2, 1), r), new(big.Rat).Mul(du, dv))
		curvature := new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).Add(uTerm, vTerm), mixed), big.NewRat(8, 1))
		curvatureSquared = new(big.Rat).Mul(norm2, curvature.Mul(curvature, curvature))
		if new(big.Rat).Mul(big.NewRat(4, 1), curvatureSquared).Cmp(tol2) <= 0 {
			break
		}
		if uTerm.Cmp(vTerm) >= 0 {
			nu *= 2
		} else {
			nv *= 2
		}
	}
	if nu > o.MaxSegments || nv > o.MaxSegments {
		return empty, geom.ErrConvergence
	}
	rows, vertexCount, triangleCount := nu, nu*nv, 2*nu*nv
	if !full {
		rows, vertexCount, triangleCount = nu+1, (nu+1)*nv+2, 2*nu*nv+2*nv
	}
	if vertexCount > o.MaxVertices || triangleCount > o.MaxTriangles {
		return empty, geom.ErrConvergence
	}
	work := 0
	piMid, piErr, e := certifiedSweepPi(ctx, o, &work)
	if e != nil {
		return empty, e
	}
	periodic := func(i, n int) (torusSample, error) {
		if i%(n/4) == 0 {
			k := i / (n / 4)
			return torusSample{
				big.NewRat([4]int64{0, 1, 0, -1}[k], 1),
				big.NewRat([4]int64{1, 0, -1, 0}[k], 1),
				new(big.Rat),
			}, nil
		}
		if i > n/2 {
			i -= n
		}
		f := big.NewRat(int64(2*i), int64(n))
		sn, cs, err, e := revolutionTrigTaylor(ctx, new(big.Rat).Mul(f, piMid), o, &work)
		if e == nil {
			err.Add(err, new(big.Rat).Mul(new(big.Rat).Abs(f), piErr))
		}
		return torusSample{sn, cs, err}, e
	}
	meridian := make([]torusSample, nv)
	for j := range meridian {
		meridian[j], e = periodic(j, nv)
		if e != nil {
			return empty, e
		}
	}
	result := CertifiedRevolvedCircleMesh{
		Solid: source, FrameDeterminant: det, AngularSegments: nu, MeridianSegments: nv,
		DisplayVertexSquaredError: new(big.Rat),
	}
	put := func(q, err [3]*big.Rat) error {
		var p [3]float64
		squared := new(big.Rat)
		for j := 0; j < 3; j++ {
			v := new(big.Rat).Set(origin[j])
			b := new(big.Rat)
			for k := 0; k < 3; k++ {
				v.Add(v, new(big.Rat).Mul(m[k][j], q[k]))
				b.Add(b, new(big.Rat).Mul(new(big.Rat).Abs(m[k][j]), err[k]))
			}
			p[j], _ = v.Float64()
			if !finite(p[j]) {
				return geom.ErrConvergence
			}
			b.Add(b, new(big.Rat).Abs(new(big.Rat).Sub(v, ratInset(p[j]))))
			squared.Add(squared, b.Mul(b, b))
		}
		if squared.Cmp(result.DisplayVertexSquaredError) > 0 {
			result.DisplayVertexSquaredError.Set(squared)
		}
		result.Mesh.Vertices = append(result.Mesh.Vertices, geom.Vec3{X: p[0], Y: p[1], Z: p[2]})
		return nil
	}
	var endSamples [2]torusSample
	for i := 0; i < rows; i++ {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		var u torusSample
		if full {
			u, e = periodic(i, nu)
		} else {
			a := new(big.Rat).Add(lo, new(big.Rat).Mul(uUpper, big.NewRat(int64(i), int64(nu))))
			u.sn, u.cs, u.err, e = revolutionTrigTaylor(ctx, a, o, &work)
		}
		if e != nil {
			return empty, e
		}
		if i == 0 {
			endSamples[0] = u
		}
		if i == rows-1 {
			endSamples[1] = u
		}
		for _, v := range meridian {
			rho := new(big.Rat).Add(radius, new(big.Rat).Mul(r, v.cs))
			eV := new(big.Rat).Mul(r, v.err)
			q := [3]*big.Rat{new(big.Rat).Mul(rho, u.cs), new(big.Rat).Mul(rho, u.sn), new(big.Rat).Add(z, new(big.Rat).Mul(r, v.sn))}
			bound := func(a *big.Rat) *big.Rat {
				return new(big.Rat).Add(new(big.Rat).Mul(rmax, u.err), new(big.Rat).Mul(new(big.Rat).Abs(a), eV))
			}
			if e := put(q, [3]*big.Rat{bound(u.cs), bound(u.sn), eV}); e != nil {
				return empty, e
			}
		}
	}
	add := func(t [3]int, owner int) {
		result.Mesh.Triangles = append(result.Mesh.Triangles, t)
		result.OriginalFace = append(result.OriginalFace, owner)
	}
	for i := 0; i < nu; i++ {
		for j := 0; j < nv; j++ {
			next, k := (i+1)%rows, (j+1)%nv
			a, b, c, d := i*nv+j, next*nv+j, next*nv+k, i*nv+k
			add([3]int{a, b, c}, 0)
			add([3]int{a, c, d}, 0)
		}
	}
	if !full {
		// The disk's circle-to-chord homotopy extends radially to each
		// triangle fan; the same meridian interpolation term bounds it.
		for end, u := range endSamples {
			center := len(result.Mesh.Vertices)
			q := [3]*big.Rat{new(big.Rat).Mul(radius, u.cs), new(big.Rat).Mul(radius, u.sn), z}
			b := new(big.Rat).Mul(radius, u.err)
			if e := put(q, [3]*big.Rat{b, b, new(big.Rat)}); e != nil {
				return empty, e
			}
			row := end * nu
			for j := 0; j < nv; j++ {
				a, b := row*nv+j, row*nv+(j+1)%nv
				if end == 1 {
					a, b = b, a
				}
				add([3]int{center, a, b}, end+1)
			}
		}
	}
	result.TrigTerms = work
	result.IdealBoundarySquaredDistance = new(big.Rat).Mul(big.NewRat(2, 1),
		new(big.Rat).Add(curvatureSquared, result.DisplayVertexSquaredError))
	if result.IdealBoundarySquaredDistance.Cmp(tol2) > 0 {
		return empty, geom.ErrConvergence
	}
	result.SignedDisplayVolume, result.Collision, e = checkClosedPeriodicMesh(ctx, result.Mesh, o.Collision)
	if e != nil {
		return empty, e
	}
	return result, nil
}
