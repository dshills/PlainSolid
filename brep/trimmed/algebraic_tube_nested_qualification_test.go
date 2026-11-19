package trimmed

func nestedTubeSectionOptions() AlgebraicTrimmedPairSectionOptions {
	o := AlgebraicTrimmedPairSectionOptions{MaxSegments: 256}
	o.Clip.MaxOperations = 512000000
	o.Clip.Loops.MaxWork = 16000000
	o.Trace.Branches.PruneSeparatedEndpoints = true
	o.Trace.Branches.MaxComparisons = 4096
	o.Trace.MaxTopologyPairs = 1000000
	o.Trace.Coverage.MaxCandidateTests = 4096
	o.Trace.Coverage.Isolation.ParameterTolerance = .125
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	return o
}
