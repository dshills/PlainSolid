package trimmed

// EllipseParameters returns immutable represented coefficients so exact
// exchange writers preserve analytic trims without fitting sampled points.
func (c EllipseUV) EllipseParameters() EllipseUV { return c }
