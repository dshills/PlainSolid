//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// AlgebraicGraphOccurrence retains a proved native common and an exact rational
// affine pose without changing its source graphs, model edges or native IDs.
// It is NOT a sewn world-space Solid. Only fiber-preserving nonsingular poses
// are accepted, with explicitly selected world projection axes. All fields are
// private; constructor inputs and returned rational poses are independently owned.
type AlgebraicGraphOccurrence struct {
	source                AlgebraicShellBoolean
	query                 *AlgebraicGraphMaterialQuery
	pose, inverse         [3][4]*big.Rat
	axes                  [2]int
	residual              int
	preparationOperations int
}

type graphOccurrenceArithmetic struct {
	ctx     context.Context
	options geom.ExactCurvePlaneEventOptions
	used    int
	err     error
}

func (a *graphOccurrenceArithmetic) calc(op byte, x, y *big.Rat) *big.Rat {
	r := new(big.Rat)
	if a.err != nil {
		return r
	}
	if e := a.ctx.Err(); e != nil {
		a.err = e
		return r
	}
	if x == nil || y == nil {
		a.err = geom.ErrInvalid
		return r
	}
	if a.used >= a.options.MaxOperations || !graphPolynomialBits(x, a.options.MaxCoefficientBits) || !graphPolynomialBits(y, a.options.MaxCoefficientBits) {
		a.err = geom.ErrConvergence
		return r
	}
	a.used++
	switch op {
	case '+':
		r.Add(x, y)
	case '-':
		r.Sub(x, y)
	case '*':
		r.Mul(x, y)
	case '/':
		if y.Sign() == 0 {
			a.err = geom.ErrConvergence
			return r
		}
		r.Quo(x, y)
	}
	if !graphPolynomialBits(r, a.options.MaxCoefficientBits) {
		a.err = geom.ErrConvergence
	}
	return r
}
func occurrenceAxesValid(axes [2]int) bool {
	return axes[0] >= 0 && axes[0] < 3 && axes[1] >= 0 && axes[1] < 3 && axes[0] != axes[1]
}
func prepareGraphOccurrencePose(a *graphOccurrenceArithmetic, pose [3][4]*big.Rat, sourceAxes, worldAxes [2]int) (owned, inverse [3][4]*big.Rat, err error) {
	if !occurrenceAxesValid(sourceAxes) || !occurrenceAxesValid(worldAxes) {
		return owned, inverse, geom.ErrInvalid
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < 4; j++ {
			if pose[i][j] == nil {
				return owned, inverse, geom.ErrInvalid
			}
			if !graphPolynomialBits(pose[i][j], a.options.MaxCoefficientBits) {
				return owned, inverse, geom.ErrConvergence
			}
			owned[i][j] = new(big.Rat).Set(pose[i][j])
		}
	}
	sr, wr := 3-sourceAxes[0]-sourceAxes[1], 3-worldAxes[0]-worldAxes[1]
	if owned[worldAxes[0]][sr].Sign() != 0 || owned[worldAxes[1]][sr].Sign() != 0 || owned[wr][sr].Sign() == 0 {
		return owned, inverse, geom.ErrConvergence
	}
	var cofactor [3][3]*big.Rat
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			var rows, cols []int
			for k := 0; k < 3; k++ {
				if k != i {
					rows = append(rows, k)
				}
				if k != j {
					cols = append(cols, k)
				}
			}
			v := a.calc('-', a.calc('*', owned[rows[0]][cols[0]], owned[rows[1]][cols[1]]), a.calc('*', owned[rows[0]][cols[1]], owned[rows[1]][cols[0]]))
			if (i+j)%2 != 0 {
				v.Neg(v)
			}
			cofactor[i][j] = v
		}
	}
	det := new(big.Rat)
	for j := 0; j < 3; j++ {
		det = a.calc('+', det, a.calc('*', owned[0][j], cofactor[0][j]))
	}
	if a.err != nil {
		return owned, inverse, a.err
	}
	if det.Sign() == 0 {
		return owned, inverse, geom.ErrConvergence
	}
	for i := 0; i < 3; i++ {
		inverse[i][3] = new(big.Rat)
		for j := 0; j < 3; j++ {
			inverse[i][j] = a.calc('/', cofactor[j][i], det)
			inverse[i][3] = a.calc('-', inverse[i][3], a.calc('*', inverse[i][j], owned[j][3]))
		}
	}
	return owned, inverse, a.err
}

// NewAlgebraicGraphOccurrence proves the exact inverse and prepares the original
// native material query. o.Projection selects the ORIGINAL graph projection;
// worldAxes selects the image projection. Pose and source preparation share
// o.Exact's cumulative work budget. A negative residual scale reverses the fiber
// endpoint order but does not alter original operand/face/edge identity.
func NewAlgebraicGraphOccurrence(ctx context.Context, s AlgebraicShellBoolean, pose [3][4]*big.Rat, worldAxes [2]int, o AlgebraicGraphMaterialOptions) (*AlgebraicGraphOccurrence, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	exact, e := graphMaterialLimits(o.Exact)
	if e != nil {
		return nil, e
	}
	sourceAxes := [2]int{0, 1}
	if o.Projection != nil {
		sourceAxes = *o.Projection
	}
	arithmetic := graphOccurrenceArithmetic{ctx: ctx, options: exact}
	p, inverse, e := prepareGraphOccurrencePose(&arithmetic, pose, sourceAxes, worldAxes)
	if e != nil {
		return nil, e
	}
	o.Exact = exact
	o.Exact.MaxOperations -= arithmetic.used
	if o.Exact.MaxOperations < 1 {
		return nil, geom.ErrConvergence
	}
	q, e := s.PrepareGraphMaterialQuery(ctx, o)
	if e != nil {
		return nil, e
	}
	return &AlgebraicGraphOccurrence{source: s, query: q, pose: p, inverse: inverse, axes: worldAxes, residual: 3 - worldAxes[0] - worldAxes[1], preparationOperations: arithmetic.used + q.PreparationOperations()}, nil
}
func (o *AlgebraicGraphOccurrence) Pose() (p [3][4]*big.Rat) {
	if o == nil || o.query == nil {
		return p
	}
	for i := range p {
		for j := range p[i] {
			p[i][j] = new(big.Rat).Set(o.pose[i][j])
		}
	}
	return p
}
func (o *AlgebraicGraphOccurrence) ProjectionAxes() [2]int {
	if o == nil || o.query == nil {
		return [2]int{-1, -1}
	}
	return o.axes
}
func (o *AlgebraicGraphOccurrence) PreparationOperations() int {
	if o == nil {
		return 0
	}
	return o.preparationOperations
}

// LocateRational classifies an exact world point through the exact inverse.
// Returned faces/rim edges remain original native Boolean IDs. Lower/Upper are
// WORLD residual-coordinate bounds above this world projected point. Mapping,
// source classification, and endpoint conversion share one exact query budget;
// failure publishes no classification or partial ancestry.
func (o *AlgebraicGraphOccurrence) LocateRational(ctx context.Context, point [3]*big.Rat, limits geom.ExactCurvePlaneEventOptions) (out AlgebraicGraphMaterialPoint, err error) {
	out.Faces = [2]int{-1, -1}
	defer func() {
		if err != nil {
			out = AlgebraicGraphMaterialPoint{Faces: [2]int{-1, -1}, ExactOperations: out.ExactOperations}
		}
	}()
	if ctx == nil || o == nil || o.query == nil {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	limits, err = graphMaterialLimits(limits)
	if err != nil {
		return out, err
	}
	a := graphOccurrenceArithmetic{ctx: ctx, options: limits}
	var sourcePoint [3]*big.Rat
	for i := 0; i < 3; i++ {
		sourcePoint[i] = new(big.Rat).Set(o.inverse[i][3])
		for j := 0; j < 3; j++ {
			sourcePoint[i] = a.calc('+', sourcePoint[i], a.calc('*', o.inverse[i][j], point[j]))
		}
	}
	if a.err != nil {
		out.ExactOperations = a.used
		return out, a.err
	}
	remaining := limits
	remaining.MaxOperations -= a.used
	if remaining.MaxOperations < 1 {
		out.ExactOperations = a.used
		return out, geom.ErrConvergence
	}
	out, err = o.query.LocateRational(ctx, sourcePoint, remaining)
	a.used += out.ExactOperations
	out.ExactOperations = a.used
	if err != nil {
		return out, err
	}
	if out.Lower != nil && out.Upper != nil {
		offset := new(big.Rat).Set(o.pose[o.residual][3])
		for _, axis := range o.query.axes {
			offset = a.calc('+', offset, a.calc('*', o.pose[o.residual][axis], sourcePoint[axis]))
		}
		out.Lower = a.calc('+', offset, a.calc('*', o.pose[o.residual][o.query.residual], out.Lower))
		out.Upper = a.calc('+', offset, a.calc('*', o.pose[o.residual][o.query.residual], out.Upper))
		if out.Lower.Cmp(out.Upper) > 0 {
			out.Lower, out.Upper = out.Upper, out.Lower
		}
	}
	out.ExactOperations = a.used
	if a.err != nil {
		return out, a.err
	}
	return out, nil
}
