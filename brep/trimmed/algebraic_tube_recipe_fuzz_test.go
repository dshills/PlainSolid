//go:build research

package trimmed

import (
	"bytes"
	"context"
	"testing"

	"plainsolid/geom"
)

func FuzzAlgebraicTubeRecipe(f *testing.F) {
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"version":1,"kind":"trimmed-algebraic-pair-section","charts":[{},{}],"faces":[{"Outer":[{},{}]},{"Outer":[{},{}]}]}`))
	f.Add([]byte(`{"version":1,"kind":"trimmed-algebraic-tube-graph-difference","base":{},"section":{},"minuend":0,"exterior":true}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<16 {
			return
		}
		ctx := context.Background()
		limits := JSONLimits{MaxBytes: 1 << 16, MaxEntities: 16, MaxControlPoints: 32}
		o := AlgebraicTrimmedPairSectionOptions{MaxSegments: 8, MaxIntervals: 16}
		o.Clip.MaxOperations = 512
		o.Trace.Seeds.Patches = geom.SurfacePatchOptions{MaxPoles: 32, MaxOperations: 512}
		o.Trace.Branches.Curves.Patches = o.Trace.Seeds.Patches
		o.Trace.Coverage.Patches = o.Trace.Seeds.Patches
		o.Trace.Coverage.MaxCandidateTests = 16
		o.Trace.Branches.MaxComparisons = 16
		o.Trace.MaxTopologyPairs = 16
		_, _ = ReadTrimmedSplinePairSectionJSON(ctx, bytes.NewReader(b), limits, o)
		_, _ = ReadAlgebraicTubeGraphDifferenceJSON(ctx, bytes.NewReader(b), limits, o,
			AlgebraicTubeGraphBooleanOptions{Patch: geom.SurfacePatchOptions{MaxOperations: 512}})
	})
}
