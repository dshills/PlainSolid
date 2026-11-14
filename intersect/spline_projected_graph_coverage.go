package intersect

// CommonXYProjectedCoverageOptions explicitly selects the exact common-XY
// projected-strata proof before generic cell coverage. MaxOperations (default
// 64M, maximum512M) is cumulative across all local candidate proof attempts,
// including failed projected attempts and generic/other enabled fallbacks.
// Original patch recovery and independent isolation retain their own limits.
type CommonXYProjectedCoverageOptions struct{ MaxOperations int }

// CommonXYProjectedCoverage reports strategy use without replacing original
// isolation/duplicate/branch diagnostics. Indices refer to Isolation.Candidates.
// Reused generic certificates remain logically charged; projected proofs are
// always fresh and no caller-owned report or cache entry is trusted.
type CommonXYProjectedCoverage struct {
	Projected, GenericFallback []int
	ExactOperations            int
}
