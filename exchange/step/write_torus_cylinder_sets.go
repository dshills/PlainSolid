package step

import (
	"context"
	"fmt"
	"io"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"strings"
	"unicode/utf8"
)

// Emit the exact decimal expansion of a represented binary scalar, so proof
// bounds do not silently omit shortest-decimal source-coordinate perturbation.
func torusCylinderSTEPReal(x float64) string {
	q := new(big.Rat).SetFloat64(x)
	digits := q.Denom().BitLen() - 1 // a reduced binary rational has denominator 2^k
	s := q.FloatString(digits)
	if digits == 0 {
		return s + "."
	}
	s = strings.TrimRight(s, "0")
	return s
}

// TorusCylinderBody describes the original canonical transverse source pair.
// Caps must lie strictly beyond the complete torus, as in the native factories.
type TorusCylinderBody struct {
	Name                                                   string
	MajorRadius, MinorRadius, CylinderRadius, Lower, Upper float64
	Operation                                              geom.TorusCylinderOperation
}
type TorusCylinderModel struct {
	Bodies                      []TorusCylinderBody
	LengthUnit, ExportTolerance float64
}

// TorusCylinderExportProof bounds model-space perturbation of exported circular
// rims and periodic chart seam offsets. It is not a certificate of a downstream
// STEP reader's healing or numerical evaluator. Returned rationals are owned.
type TorusCylinderExportProof struct {
	Height                                                                           float64
	HeightSquared, HeightError, TorusResidualDistance, PeriodicSeamError, ErrorUpper *big.Rat
}

func CertifyTorusCylinderExport(b TorusCylinderBody, tolerance float64) (TorusCylinderExportProof, error) {
	var zero TorusCylinderExportProof
	if !utf8.ValidString(b.Name) || math.IsNaN(tolerance) || math.IsInf(tolerance, 0) || tolerance <= 0 {
		return zero, geom.ErrInvalid
	}
	var h2 *big.Rat
	if b.Operation == geom.TorusCylinderUnion {
		s, e := parametric.NewTorusCylinderUnionSet(context.Background(), b.MajorRadius, b.MinorRadius, b.CylinderRadius, b.Lower, b.Upper, tolerance)
		if e != nil {
			return zero, e
		}
		_, h2, _, _ = s.RimProof()
	} else {
		s, e := parametric.NewTorusCylinderSet(context.Background(), b.MajorRadius, b.MinorRadius, b.CylinderRadius, b.Lower, b.Upper, b.Operation, tolerance)
		if e != nil {
			return zero, e
		}
		_, h2, _, _ = s.RimProof()
	}
	hf, _ := h2.Float64()
	h := math.Sqrt(hf)
	if h <= 0 || math.IsInf(h, 0) {
		return zero, geom.ErrConvergence
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	residual := new(big.Rat).Abs(new(big.Rat).Sub(new(big.Rat).Mul(q(h), q(h)), h2))
	he := new(big.Rat).Quo(residual, q(h))             // |h-sqrt(h2)| <= |h²-h2|/h.
	te := new(big.Rat).Quo(residual, q(b.MinorRadius)) // distance to meridian circle.
	// A decimal rational enclosure of pi; one last-place unit separates its
	// endpoints. This controls the existing STEP convention of represented 2pi.
	piLo, _ := new(big.Rat).SetString("3.14159265358979323846264338327950288419716939937510")
	unit, _ := new(big.Rat).SetString("0.00000000000000000000000000000000000000000000000001")
	piHi := new(big.Rat).Add(piLo, unit)
	a := new(big.Rat).Abs(new(big.Rat).Sub(q(2*math.Pi), new(big.Rat).Mul(piLo, big.NewRat(2, 1))))
	z := new(big.Rat).Abs(new(big.Rat).Sub(q(2*math.Pi), new(big.Rat).Mul(piHi, big.NewRat(2, 1))))
	if z.Cmp(a) > 0 {
		a = z
	}
	pe := new(big.Rat).Mul(a, new(big.Rat).Add(q(b.MajorRadius), q(b.MinorRadius)))
	total := new(big.Rat).Add(he, te)
	total.Add(total, pe)
	if total.Cmp(q(tolerance)) > 0 {
		return zero, geom.ErrConvergence
	}
	return TorusCylinderExportProof{h, h2, he, te, pe, total}, nil
}

// WriteTorusCylinderSets writes analytic STEP BReps with circular model curves,
// original support surfaces and explicit periodic seams. Finite STEP reals
// cannot encode irrational rim heights exactly: ExportTolerance is mandatory,
// and CertifyTorusCylinderExport proves the original-support residual beforehand.
// No sampled geometry or rounded meridian angle is authored. Native identity
// remains the exact original pair, independently reconstructible from inputs.
func WriteTorusCylinderSets(w io.Writer, m TorusCylinderModel) error {
	if w == nil || len(m.Bodies) == 0 || len(m.Bodies) > 1024 {
		return geom.ErrInvalid
	}
	proofs := make([]TorusCylinderExportProof, len(m.Bodies))
	for i, b := range m.Bodies {
		p, e := CertifyTorusCylinderExport(b, m.ExportTolerance)
		if e != nil {
			return e
		}
		proofs[i] = p
	}
	return writeStandalone(w, m.LengthUnit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		point := func(p geom.Vec3) int64 {
			return emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s,%s))", torusCylinderSTEPReal(p.X), torusCylinderSTEPReal(p.Y), torusCylinderSTEPReal(p.Z)))
		}
		dir := func(p geom.Vec3) int64 {
			return emit(fmt.Sprintf("DIRECTION('',(%s,%s,%s))", torusCylinderSTEPReal(p.X), torusCylinderSTEPReal(p.Y), torusCylinderSTEPReal(p.Z)))
		}
		place := func(p, x, z geom.Vec3) int64 {
			return emit(fmt.Sprintf("AXIS2_PLACEMENT_3D('',#%d,#%d,#%d)", point(p), dir(z), dir(x)))
		}
		truth := func(v bool) string {
			if v {
				return ".T."
			}
			return ".F."
		}
		out := make([]int64, len(m.Bodies))
		for i, b := range m.Bodies {
			h := proofs[i].Height
			pl := place(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
			torus := emit(fmt.Sprintf("TOROIDAL_SURFACE('',#%d,%s,%s)", pl, torusCylinderSTEPReal(b.MajorRadius), torusCylinderSTEPReal(b.MinorRadius)))
			wall := emit(fmt.Sprintf("CYLINDRICAL_SURFACE('',#%d,%s)", pl, torusCylinderSTEPReal(b.CylinderRadius)))
			ctx := emit("(GEOMETRIC_REPRESENTATION_CONTEXT(2) PARAMETRIC_REPRESENTATION_CONTEXT() REPRESENTATION_CONTEXT('',''))")
			seamPC := func(sid int64, u float64) int64 {
				p := emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,0.))", torusCylinderSTEPReal(u)))
				d := emit("DIRECTION('',(0.,1.))")
				v := emit(fmt.Sprintf("VECTOR('',#%d,1.)", d))
				line := emit(fmt.Sprintf("LINE('',#%d,#%d)", p, v))
				rep := emit(fmt.Sprintf("DEFINITIONAL_REPRESENTATION('',(#%d),#%d)", line, ctx))
				return emit(fmt.Sprintf("PCURVE('',#%d,#%d)", sid, rep))
			}
			edge := func(a, z, curve int64) int64 { return emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,.T.)", a, z, curve)) }
			seam := func(sid, curve, a, z int64) int64 {
				pc0, pc1 := seamPC(sid, 0), seamPC(sid, 2*math.Pi)
				sc := emit(fmt.Sprintf("SEAM_CURVE('',#%d,(#%d,#%d),.CURVE_3D.)", curve, pc0, pc1))
				return edge(a, z, sc)
			}
			ring := func(z float64) (int64, int64) {
				v := emit(fmt.Sprintf("VERTEX_POINT('',#%d)", point(geom.Vec3{X: b.CylinderRadius, Z: z})))
				c := emit(fmt.Sprintf("CIRCLE('',#%d,%s)", place(geom.Vec3{Z: z}, geom.Vec3{X: 1}, geom.Vec3{Z: 1}), torusCylinderSTEPReal(b.CylinderRadius)))
				return v, edge(v, v, c)
			}
			vlo, elo := ring(-h)
			vhi, ehi := ring(h)
			oriented := func(e int64, sense bool) int64 {
				return emit(fmt.Sprintf("ORIENTED_EDGE('',*,*,#%d,%s)", e, truth(sense)))
			}
			face := func(sid int64, edges []int64, sense bool) int64 {
				loop := emit("EDGE_LOOP(''," + references(edges) + ")")
				bound := emit(fmt.Sprintf("FACE_OUTER_BOUND('',#%d,.T.)", loop))
				return emit(fmt.Sprintf("ADVANCED_FACE('',(#%d),#%d,%s)", bound, sid, truth(sense)))
			}
			meridian := emit(fmt.Sprintf("CIRCLE('',#%d,%s)", place(geom.Vec3{X: b.MajorRadius}, geom.Vec3{X: 1}, geom.Vec3{Y: -1}), torusCylinderSTEPReal(b.MinorRadius)))
			start, end, first, last := vlo, vhi, elo, ehi
			if b.Operation == geom.TorusCylinderCommon {
				start, end, first, last = vhi, vlo, ehi, elo
			}
			ts := seam(torus, meridian, start, end)
			faces := []int64{face(torus, []int64{oriented(first, true), oriented(ts, true), oriented(last, false), oriented(ts, false)}, true)}
			line := emit(fmt.Sprintf("LINE('',#%d,#%d)", point(geom.Vec3{X: b.CylinderRadius}), emit(fmt.Sprintf("VECTOR('',#%d,1.)", dir(geom.Vec3{Z: 1})))))
			band := func(a, z, ea, ez int64, inward bool) int64 {
				es := seam(wall, line, a, z)
				uses := []int64{oriented(ea, true), oriented(es, true), oriented(ez, false), oriented(es, false)}
				if inward {
					uses = []int64{oriented(es, true), oriented(ez, true), oriented(es, false), oriented(ea, false)}
				}
				return face(wall, uses, !inward)
			}
			if b.Operation != geom.TorusCylinderUnion {
				faces = append(faces, band(vlo, vhi, elo, ehi, b.Operation == geom.TorusCylinderDifference))
			} else {
				vl, el := ring(b.Lower)
				vu, eu := ring(b.Upper)
				faces = append(faces, band(vl, vlo, el, elo, false), band(vhi, vu, ehi, eu, false))
				for j, z := range []float64{b.Lower, b.Upper} {
					p := emit(fmt.Sprintf("PLANE('',#%d)", place(geom.Vec3{Z: z}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})))
					e := el
					if j == 1 {
						e = eu
					}
					faces = append(faces, face(p, []int64{oriented(e, j == 1)}, j == 1))
				}
			}
			shell := emit("CLOSED_SHELL(''," + references(faces) + ")")
			out[i] = emit(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(b.Name), shell))
		}
		return out
	})
}
