package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// NativePrismDispatchOptions bounds independent stages. Classify proves the
// complete Common and source relation; Construct proves the selected family;
// Publish bounds transition materialization and aggregate output cardinality.
// There are no speculative retries or generic BSP fallback.
type NativePrismDispatchOptions struct {
	Classify, Construct NativePrismBooleanOptions
	Publish             AffineArrangementPrismOptions
}
type NativePrismMaterialKind uint8

const (
	NativePrismMaterialEmpty NativePrismMaterialKind = iota
	NativePrismMaterialContact
	NativePrismMaterialSolid
)

type NativePrismMaterialFamily uint8

const (
	NativePrismFamilyCommon NativePrismMaterialFamily = iota
	NativePrismFamilyMatching
	NativePrismFamilyBlindCut
	NativePrismFamilyCavityCut
	NativePrismFamilyThroughCut
	NativePrismFamilyBossFuse
	NativePrismFamilyCrossingCut
	NativePrismFamilyCrossingFuse
	NativePrismFamilyOriginal
	NativePrismFamilyEmpty
)

// NativePrismMaterial is immutable. Typed accessors identify the actual exact
// construction proof; no generic single-source BSP face history is synthesized.
// SourceOrder maps the selected family's original operands to caller operands.
type NativePrismMaterial struct {
	owners             [2]brep.Solid
	components         []brep.Solid
	componentOwners    []int
	order              [2]int
	kind               NativePrismMaterialKind
	family             NativePrismMaterialFamily
	common             *NativePrismIntervalCommon
	matching           *NativePrismBoolean
	blind              *NativePrismBlindCut
	cavity             *NativePrismCavityCut
	through            *NativePrismThroughCut
	boss               *NativePrismBossFuse
	crossingCut        *NativeCrossingPrismCut
	crossingFuse       *NativeCrossingPrismFuse
	classificationWork int
}

// Complete distinguishes a published proof from a zero-value result.
func (s *NativePrismMaterial) Complete() bool {
	return s != nil && s.common != nil && s.common.profile != nil && s.common.profile.Complete()
}
func (s *NativePrismMaterial) Kind() NativePrismMaterialKind {
	if s == nil {
		return NativePrismMaterialEmpty
	}
	return s.kind
}
func (s *NativePrismMaterial) Family() NativePrismMaterialFamily {
	if s == nil {
		return NativePrismFamilyEmpty
	}
	return s.family
}
func (s *NativePrismMaterial) Components() []brep.Solid {
	if s == nil {
		return nil
	}
	return append([]brep.Solid(nil), s.components...)
}
func (s *NativePrismMaterial) OriginalOwner(i int) (brep.Solid, error) {
	if s == nil || i < 0 || i > 1 {
		return brep.Solid{}, geom.ErrInvalid
	}
	return s.owners[i], nil
}
func (s *NativePrismMaterial) SourceOrder() [2]int {
	if s == nil {
		return [2]int{-1, -1}
	}
	return s.order
}

// ComponentOwners is populated only for unchanged original components, giving
// caller operand IDs. Constructed components instead use their typed proof.
func (s *NativePrismMaterial) ComponentOwners() []int {
	if s == nil {
		return nil
	}
	return append([]int(nil), s.componentOwners...)
}
func (s *NativePrismMaterial) CommonProof() *NativePrismIntervalCommon {
	if s == nil {
		return nil
	}
	return s.common
}
func (s *NativePrismMaterial) MatchingProof() *NativePrismBoolean {
	if s == nil {
		return nil
	}
	return s.matching
}
func (s *NativePrismMaterial) BlindCutProof() *NativePrismBlindCut {
	if s == nil {
		return nil
	}
	return s.blind
}
func (s *NativePrismMaterial) CavityCutProof() *NativePrismCavityCut {
	if s == nil {
		return nil
	}
	return s.cavity
}
func (s *NativePrismMaterial) ThroughCutProof() *NativePrismThroughCut {
	if s == nil {
		return nil
	}
	return s.through
}
func (s *NativePrismMaterial) BossFuseProof() *NativePrismBossFuse {
	if s == nil {
		return nil
	}
	return s.boss
}
func (s *NativePrismMaterial) CrossingCutProof() *NativeCrossingPrismCut {
	if s == nil {
		return nil
	}
	return s.crossingCut
}
func (s *NativePrismMaterial) CrossingFuseProof() *NativeCrossingPrismFuse {
	if s == nil {
		return nil
	}
	return s.crossingFuse
}
func (s *NativePrismMaterial) ClassificationNativeOperations() int {
	if s == nil {
		return 0
	}
	return s.classificationWork
}

// CombineNativePrisms selects an authenticated cardinal prism material family.
// Common preserves empty/contact/positive-material distinctions. Difference with
// no positive-volume Common preserves stock; complete inclusion can prove empty
// Difference or an unchanged Union owner. Disjoint Union keeps two original
// components. Other unsupported contacts or axial/profile regimes return an
// error, never a partial result or an inferred generic BSP history.
func CombineNativePrisms(ctx context.Context, a, b brep.Solid, operation intersect.AffineRingOperation, o NativePrismDispatchOptions) (*NativePrismMaterial, error) {
	if ctx == nil || operation > intersect.AffineRingDifference {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	components, vertices := o.Publish.MaxComponents, o.Publish.MaxVertices
	if components == 0 {
		components = 64
	}
	if vertices == 0 {
		vertices = 4096
	}
	if components < 1 || components > 256 || vertices < 6 || vertices > 16384 || o.Publish.MaxOperations < 0 || o.Publish.MaxOperations > 128000000 || o.Publish.MaxCoefficientBits < 0 || o.Publish.MaxCoefficientBits > 65536 || (o.Publish.MaxCoefficientBits > 0 && o.Publish.MaxCoefficientBits < 64) {
		return nil, geom.ErrInvalid
	}
	// Prefer a matching-height representation, then a one-ended transition,
	// then a full cover, finally interior spans. All candidates first pass the
	// complete original prism authentication; these comparisons propose an axis,
	// while the complete Common and selected constructor still prove geometry.
	selected, best := -1, 99
	sourceLimit := o.Classify.Native.MaxVertices
	if sourceLimit == 0 {
		sourceLimit = 4096
	}
	if sourceLimit < 6 || sourceLimit > 16384 {
		return nil, geom.ErrInvalid
	}
	for axis := 0; axis < 3; axis++ {
		x, e := authenticateNativePrism(ctx, a, axis, sourceLimit)
		if e != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		y, e := authenticateNativePrism(ctx, b, axis, sourceLimit)
		if e != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		rank := 3
		if x.lo == y.lo && x.hi == y.hi {
			rank = 0
		} else if (x.lo < y.lo && y.lo < x.hi && x.hi < y.hi) || (y.lo < x.lo && x.lo < y.hi && y.hi < x.hi) {
			rank = 1
		} else if y.lo <= x.lo && x.hi <= y.hi {
			rank = 2
		}
		if rank < best {
			selected, best = axis, rank
		}
	}
	if selected < 0 {
		return nil, geom.ErrDomain
	}
	common, e := commonNativePrismIntervalsAxis(ctx, a, b, o.Classify, selected)
	if e != nil {
		return nil, e
	}
	out := &NativePrismMaterial{owners: [2]brep.Solid{a, b}, order: [2]int{0, 1}, common: common, classificationWork: common.NativeOperations()}
	finish := func() (*NativePrismMaterial, error) {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if len(out.components) > components {
			return nil, geom.ErrConvergence
		}
		n := 0
		for _, s := range out.components {
			n += len(s.Vertices())
			if n > vertices {
				return nil, geom.ErrConvergence
			}
		}
		return out, nil
	}
	original := func(ids ...int) (*NativePrismMaterial, error) {
		out.kind = NativePrismMaterialSolid
		out.family = NativePrismFamilyOriginal
		for _, i := range ids {
			out.components = append(out.components, out.owners[i])
			out.componentOwners = append(out.componentOwners, i)
		}
		return finish()
	}
	empty := func() (*NativePrismMaterial, error) {
		out.kind = NativePrismMaterialEmpty
		out.family = NativePrismFamilyEmpty
		return finish()
	}
	if operation == intersect.AffineRingIntersection {
		out.family = NativePrismFamilyCommon
		switch common.kind {
		case NativePrismCommonEmpty:
			out.kind = NativePrismMaterialEmpty
		case NativePrismCommonContact:
			out.kind = NativePrismMaterialContact
		case NativePrismCommonMaterial:
			out.kind = NativePrismMaterialSolid
			out.components = common.material.Components()
		default:
			return nil, geom.ErrInvalid
		}
		return finish()
	}
	if operation == intersect.AffineRingDifference && common.kind != NativePrismCommonMaterial {
		return original(0)
	}
	if operation == intersect.AffineRingUnion && common.kind == NativePrismCommonEmpty {
		return original(0, 1)
	}
	maxVertices := o.Classify.Native.MaxVertices
	if maxVertices == 0 {
		maxVertices = 4096
	}
	profiles := [2]nativePrismProfile{}
	for i := range profiles {
		p, e := authenticateNativePrism(ctx, out.owners[i], common.axis, maxVertices)
		if e != nil {
			return nil, e
		}
		profiles[i] = p
	}
	x, y := profiles[0], profiles[1]
	if x.lo == y.lo && x.hi == y.hi {
		out.matching, e = CombineMatchingNativePrisms(ctx, a, b, operation, o.Construct)
		if e != nil {
			return nil, e
		}
		out.family = NativePrismFamilyMatching
		out.components = out.matching.Material().Components()
		out.kind = NativePrismMaterialSolid
		if len(out.components) == 0 {
			out.kind = NativePrismMaterialEmpty
		}
		return finish()
	}
	if common.kind != NativePrismCommonMaterial {
		return nil, geom.ErrDomain
	}
	maximum, bits := o.Classify.Native.MaxOperations, o.Classify.Native.MaxCoefficientBits
	if maximum == 0 {
		maximum = 8000000
	}
	if bits == 0 {
		bits = 16384
	}
	budget := &nativePrismAffineBudget{ctx: ctx, max: maximum, bits: bits, work: common.NativeOperations()}
	area := func(p nativePrismProfile) *big.Rat {
		sum := new(big.Rat)
		for _, ring := range append([][][2]*big.Rat{p.trim.Outer}, p.trim.Holes...) {
			for i, v := range ring {
				w := ring[(i+1)%len(ring)]
				sum = budget.op('+', sum, budget.op('-', budget.op('*', v[0], w[1]), budget.op('*', v[1], w[0])))
			}
		}
		return budget.op('/', sum, big.NewRat(2, 1))
	}
	areas := [2]*big.Rat{area(x), area(y)}
	ca := new(big.Rat)
	for _, c := range common.profile.Area().Cycles() {
		ca = budget.op('+', ca, c.SignedAreaA)
	}
	if budget.err != nil {
		return nil, budget.err
	}
	out.classificationWork = budget.work
	// The complete exact polygon Common is a subset of each authenticated region.
	// Equal positive area therefore proves regularized profile inclusion, not a
	// heuristic comparison of independently estimated solid volumes.
	aInsideB := ca.Cmp(areas[0]) == 0
	bInsideA := ca.Cmp(areas[1]) == 0
	fullA := aInsideB && y.lo <= x.lo && x.hi <= y.hi
	fullB := bInsideA && x.lo <= y.lo && y.hi <= x.hi
	if operation == intersect.AffineRingDifference && fullA {
		return empty()
	}
	if operation == intersect.AffineRingUnion {
		if fullB {
			return original(0)
		}
		if fullA {
			return original(1)
		}
	}
	out.kind = NativePrismMaterialSolid
	if operation == intersect.AffineRingDifference {
		if bInsideA {
			switch {
			case y.lo <= x.lo && x.hi <= y.hi:
				out.through, e = CutContainedNativePrismThrough(ctx, a, b, o.Construct)
				if e == nil {
					out.family = NativePrismFamilyThroughCut
					out.components = []brep.Solid{out.through.Solid()}
				}
			case x.lo < y.lo && y.hi < x.hi:
				out.cavity, e = CutContainedNativePrismCavity(ctx, a, b, o.Construct)
				if e == nil {
					out.family = NativePrismFamilyCavityCut
					out.components = []brep.Solid{out.cavity.Solid()}
				}
			default:
				out.blind, e = CutContainedNativePrismBlind(ctx, a, b, o.Construct)
				if e == nil {
					out.family = NativePrismFamilyBlindCut
					out.components = []brep.Solid{out.blind.Solid()}
				}
			}
		} else if !aInsideB {
			q, err := ImprintCrossingNativePrismCut(ctx, a, b, o.Construct)
			e = err
			if e == nil {
				out.crossingCut, e = q.BuildNative(ctx, o.Publish)
				if e == nil {
					out.family = NativePrismFamilyCrossingCut
					out.components = []brep.Solid{out.crossingCut.Solid()}
				}
			}
		} else {
			return nil, geom.ErrDomain
		}
	} else {
		if bInsideA || aInsideB {
			stock, tool := a, b
			if !bInsideA {
				stock, tool = b, a
				out.order = [2]int{1, 0}
			}
			out.boss, e = FuseContainedNativePrismBoss(ctx, stock, tool, o.Construct)
			if e == nil {
				out.family = NativePrismFamilyBossFuse
				out.components = []brep.Solid{out.boss.Solid()}
			}
		} else {
			q, err := ImprintCrossingNativePrismFuse(ctx, a, b, o.Construct)
			e = err
			if e == nil {
				out.crossingFuse, e = q.BuildNative(ctx, o.Publish)
				if e == nil {
					out.family = NativePrismFamilyCrossingFuse
					out.components = []brep.Solid{out.crossingFuse.Solid()}
				}
			}
		}
	}
	if e != nil {
		return nil, e
	}
	if len(out.components) == 0 {
		return nil, geom.ErrInvalid
	}
	return finish()
}
