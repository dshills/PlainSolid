package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// WrappedRationalStationAreaOptions requests an exact positive upper bound on
// the width of the total area enclosure. Source governs topology authentication.
// The other limits govern the shared profile subdivision and rational ledger.
type WrappedRationalStationAreaOptions struct {
	Source                                         geom.SurfacePatchOptions
	AreaTolerance                                  *big.Rat
	MaxSegments, MaxOperations, MaxCoefficientBits int
}

// WrappedRationalStationFaceArea binds an enclosure to an original solid face.
type WrappedRationalStationFaceArea struct {
	Face int
	Area WrappedStationMassInterval
}

// WrappedRationalStationAreaBounds encloses the original material boundary.
// Each wall band and both caps occur once. Seams and intermediate rims have
// zero area. Segments counts the final partition shared by all faces.
type WrappedRationalStationAreaBounds struct {
	Total                     WrappedStationMassInterval
	Faces                     []WrappedRationalStationFaceArea
	Segments, ExactOperations int
}

// BoundWrappedRationalStationArea bounds the boundary area of an authenticated
// filled station loft directly on its original weighted rational profile.
func (s Solid) BoundWrappedRationalStationArea(ctx context.Context, o WrappedRationalStationAreaOptions) (WrappedRationalStationAreaBounds, error) {
	limits, err := normalizeWrappedStationMassOptions(ctx, WrappedRationalStationMassOptions{
		Source: o.Source, VolumeTolerance: o.AreaTolerance, MaxSegments: o.MaxSegments,
		MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
	})
	if err != nil {
		return WrappedRationalStationAreaBounds{}, err
	}
	proof, bands, err := s.validateWrappedProfileStationLoft(ctx, o.Source, false)
	if err != nil {
		return WrappedRationalStationAreaBounds{}, err
	}
	return boundWrappedStationArea(ctx, []WrappedStationLoftProof{proof}, [][]int{proof.History.WallFaces}, proof.History.CapFaces, bands[0], limits)
}

// BoundWrappedRationalStationBoreArea bounds the boundary of a strictly nested
// through-bore. It adds both lateral wall families and uses the exact difference
// of squared terminal scales for each annular cap.
func (s Solid) BoundWrappedRationalStationBoreArea(ctx context.Context, o WrappedRationalStationAreaOptions) (WrappedRationalStationAreaBounds, error) {
	limits, err := normalizeWrappedStationMassOptions(ctx, WrappedRationalStationMassOptions{
		Source: o.Source, VolumeTolerance: o.AreaTolerance, MaxSegments: o.MaxSegments,
		MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
	})
	if err != nil {
		return WrappedRationalStationAreaBounds{}, err
	}
	proof, err := s.ValidateWrappedRationalStationBore(ctx, o.Source)
	if err != nil {
		return WrappedRationalStationAreaBounds{}, err
	}
	first := s.faces[proof.OuterWalls[0]].Surface.(*geom.WrappedSurfacePatch)
	return boundWrappedStationArea(ctx, []WrappedStationLoftProof{proof.Outer, proof.Inner}, [][]int{proof.OuterWalls, proof.InnerWalls}, proof.Caps, first, limits)
}

type stationAreaBand struct {
	face                              int
	heightSquared, deltaSquared, mean *big.Rat
}

func boundWrappedStationArea(ctx context.Context, proofs []WrappedStationLoftProof, walls [][]int, caps [2]int,
	first *geom.WrappedSurfacePatch, o WrappedRationalStationMassOptions) (WrappedRationalStationAreaBounds, error) {
	var empty WrappedRationalStationAreaBounds
	b := &stationMassBudget{ctx: ctx, limit: o.MaxOperations, bits: o.MaxCoefficientBits}
	tolerance := new(big.Rat).Set(b.check(o.VolumeTolerance))
	segments, _, err := stationProfileSegments(first, b)
	if err != nil {
		return empty, err
	}
	var bands []stationAreaBand
	capScale := [2]*big.Rat{new(big.Rat), new(big.Rat)}
	for i, proof := range proofs {
		for j, face := range walls[i] {
			h := b.sub(proof.Heights[j+1], proof.Heights[j])
			d := b.sub(proof.Radii[j+1], proof.Radii[j])
			mean := b.div(b.add(proof.Radii[j+1], proof.Radii[j]), big.NewRat(2, 1))
			bands = append(bands, stationAreaBand{face, b.mul(h, h), b.mul(d, d), mean})
		}
		for j, radius := range [2]*big.Rat{proof.Radii[0], proof.Radii[len(proof.Radii)-1]} {
			x := b.mul(radius, radius)
			if i == 0 {
				capScale[j] = x
			} else {
				capScale[j] = b.sub(capScale[j], x)
			}
		}
	}
	if b.err != nil {
		return empty, b.err
	}
	for {
		faces := make([]WrappedRationalStationFaceArea, 0, len(bands)+2)
		for _, band := range bands {
			faces = append(faces, WrappedRationalStationFaceArea{band.face, WrappedStationMassInterval{new(big.Rat), new(big.Rat)}})
		}
		for _, segment := range segments {
			span := stationAreaSpanFrom(segment, b)
			if b.err != nil {
				return empty, b.err
			}
			for i, band := range bands {
				faces[i].Area = b.plus(faces[i].Area, band.encloseSpan(span, b))
				if b.err != nil {
					return empty, b.err
				}
			}
		}
		capProfile, _, err := stationProfileEnclosure(segments, false, b)
		if err != nil {
			return empty, err
		}
		for i, face := range caps {
			faces = append(faces, WrappedRationalStationFaceArea{face, b.scaled(capProfile, capScale[i])})
		}
		total := WrappedStationMassInterval{new(big.Rat), new(big.Rat)}
		for _, face := range faces {
			total = b.plus(total, face.Area)
		}
		width := b.sub(total.Upper, total.Lower)
		if b.err != nil {
			return empty, b.err
		}
		if width.Cmp(tolerance) <= 0 {
			return WrappedRationalStationAreaBounds{Total: total, Faces: faces, Segments: len(segments), ExactOperations: b.used}, nil
		}
		if len(segments) > o.MaxSegments/2 {
			return empty, geom.ErrConvergence
		}
		segments = splitStationProfileSegments(segments, b)
		if b.err != nil {
			return empty, b.err
		}
	}
}

type stationAreaSpan struct {
	numerators [3][3]*big.Rat
	integral   [3]*big.Rat
	coeff      [3]*big.Rat
	remainder  *big.Rat
}

// For q=P/W, the augmented derivative (q'x,q'y,det(q,q')) has
// degree-two Bernstein numerators over W². A band with height h and scale
// change d uses their weighted Euclidean norm with factors (h,h,d).
// Integration along the station coordinate multiplies this by mean scale.
func stationAreaSpanFrom(q stationMassSegment, b *stationMassBudget) stationAreaSpan {
	var span stationAreaSpan
	for k, pair := range [3][2]int{{0, 1}, {0, 2}, {1, 2}} {
		a, c := q[pair[0]], q[pair[1]]
		for axis, wedge := range [3][2]int{{2, 0}, {2, 1}, {0, 1}} {
			i, j := wedge[0], wedge[1]
			n := b.sub(b.mul(a[i], c[j]), b.mul(a[j], c[i]))
			if k != 1 {
				n = b.add(n, n)
			}
			span.numerators[k][axis] = n
		}
	}
	w0, w1, w2 := q[0][2], q[1][2], q[2][2]
	w11 := b.mul(w1, w1)
	den := [5]*big.Rat{b.mul(w0, w0), b.mul(w0, w1), b.div(b.add(b.mul(w0, w2), b.add(w11, w11)), big.NewRat(3, 1)), b.mul(w1, w2), b.mul(w2, w2)}
	low, high := den[0], den[0]
	for _, d := range den[1:] {
		if d.Cmp(low) < 0 {
			low = d
		}
		if d.Cmp(high) > 0 {
			high = d
		}
	}
	if b.err != nil {
		return span
	}
	// For m=(low+high)/2 and D=W², 1/D = (2m-D)/m² +
	// (D-m)²/(m²D). The first term is positive. The second lies in
	// [0, ((high-low)/2)²/(m²*low)].
	twiceMean := b.add(low, high)
	mean := b.div(twiceMean, big.NewRat(2, 1))
	meanSq := b.mul(mean, mean)
	radius := b.sub(mean, low)
	span.remainder = b.div(b.mul(radius, radius), b.mul(meanSq, low))
	var linear [5]*big.Rat
	for j, d := range den {
		linear[j] = b.div(b.sub(twiceMean, d), meanSq)
	}
	// Integrate B_i^2 times each degree-four Bernstein term exactly.
	choose2 := [3]int64{1, 2, 1}
	choose4 := [5]int64{1, 4, 6, 4, 1}
	choose6 := [7]int64{1, 6, 15, 20, 15, 6, 1}
	for i := range span.coeff {
		span.coeff[i] = new(big.Rat)
		for j, d := range linear {
			integral := big.NewRat(choose2[i]*choose4[j], 7*choose6[i+j])
			span.coeff[i] = b.add(span.coeff[i], b.mul(d, integral))
		}
	}
	for axis := range span.integral {
		span.integral[axis] = new(big.Rat)
		for i, c := range span.coeff {
			span.integral[axis] = b.add(span.integral[axis], b.mul(c, span.numerators[i][axis]))
		}
	}
	return span
}

func (band stationAreaBand) encloseSpan(span stationAreaSpan, b *stationMassBudget) WrappedStationMassInterval {
	// The norm of the integrated vector is a lower bound by the triangle
	// inequality. The weighted sum of control norms is an upper bound by
	// convexity. Bound the positive reciprocal remainder by the largest norm.
	lower := stationAreaSqrt(band.normSquared(span.integral, b), false, b)
	upper, largest := new(big.Rat), new(big.Rat)
	for i, n := range span.numerators {
		norm := stationAreaSqrt(band.normSquared(n, b), true, b)
		upper = b.add(upper, b.mul(span.coeff[i], norm))
		if norm.Cmp(largest) > 0 {
			largest = norm
		}
	}
	upper = b.add(upper, b.mul(span.remainder, largest))
	return WrappedStationMassInterval{
		stationAreaDyadic(b.mul(lower, band.mean), false, b),
		stationAreaDyadic(b.mul(upper, band.mean), true, b),
	}
}

func (band stationAreaBand) normSquared(vector [3]*big.Rat, b *stationMassBudget) *big.Rat {
	x, y, z := b.mul(vector[0], vector[0]), b.mul(vector[1], vector[1]), b.mul(vector[2], vector[2])
	return b.add(b.mul(b.add(x, y), band.heightSquared), b.mul(z, band.deltaSquared))
}

const stationAreaFractionBits = 96

// Compute sqrt(q) outward on a fixed dyadic grid using integer square root.
func stationAreaSqrt(q *big.Rat, upper bool, b *stationMassBudget) *big.Rat {
	if b.err != nil || q.Sign() == 0 {
		return new(big.Rat)
	}
	numerator := new(big.Int).Lsh(q.Num(), 2*stationAreaFractionBits)
	floor := new(big.Int).Quo(numerator, q.Denom())
	root := new(big.Int).Sqrt(floor)
	if upper {
		square := new(big.Int).Mul(root, root)
		if square.Mul(square, q.Denom()).Cmp(numerator) < 0 {
			root.Add(root, big.NewInt(1))
		}
	}
	return b.check(new(big.Rat).SetFrac(root, new(big.Int).Lsh(big.NewInt(1), stationAreaFractionBits)))
}

// Rounding each lateral contribution keeps its aggregate denominator dyadic
// even when the weighted integrals of individual spans have different factors.
func stationAreaDyadic(q *big.Rat, upper bool, b *stationMassBudget) *big.Rat {
	if b.err != nil || q.Sign() == 0 {
		return new(big.Rat)
	}
	numerator := new(big.Int).Lsh(q.Num(), stationAreaFractionBits)
	intPart, rest := new(big.Int).QuoRem(numerator, q.Denom(), new(big.Int))
	if upper && rest.Sign() != 0 {
		intPart.Add(intPart, big.NewInt(1))
	}
	return b.check(new(big.Rat).SetFrac(intPart, new(big.Int).Lsh(big.NewInt(1), stationAreaFractionBits)))
}
