package geom

// NewVerifiedSurfaceTubeWithCertificate performs one uniform branch proof and
// returns both its immutable source-bound tube and its public diagnostic report.
// The report is never retained or trusted as provenance: modifying it cannot
// alter the tube. On failure the tube is zero; the report may retain exclusion,
// inconclusive bounds and actual arithmetic work, exactly as CertifySurfacePairTube.
// No caller-provided certificate is accepted. Existing per-proof limits apply.
func NewVerifiedSurfaceTubeWithCertificate(a, b *SurfacePatch, o SurfacePairTubeOptions) (VerifiedSurfaceTube, SurfaceTubeCertificate, error) {
	r, e := CertifySurfacePairTube(a, b, o)
	if e != nil {
		return VerifiedSurfaceTube{}, r, e
	}
	if !r.ProvenBranch {
		return VerifiedSurfaceTube{}, r, ErrConvergence
	}
	return VerifiedSurfaceTube{a: a, b: b, axis: o.FreeAxis, tangentSign: r.FreeTangentSign}, r, nil
}
