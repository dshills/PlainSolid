# Exact torus-inclusive SI normalization

`ReadNormalizedIndependentToroidalMaterials` extends the independently owned
planar/cylinder/sphere normalization workflow with the strictly model-only ring
torus family. At least one torus is required. The previous normalization readers
and native envelope tags remain strict; the returned shared result type does not
make the previous heterogeneous persistence API accept torus products.

The dimensional audit is deliberately restricted to the authenticated torus graph:
all 3D points, both TOROIDAL_SURFACE radii, and both CIRCLE radii scale by the exact
metre/millimetre ratio. Signed unit directions, coedge senses and graph IDs stay
unchanged. No angular pcurve, trim parameter or vendor SEAM_CURVE is admitted.
Source and normalized graphs each receive the full circle/support/vertex/quotient
proof, including R>r. Scaled coordinates, radii, their sums and offsets must remain
binary64-exact. Unrepresentable radii fail without partial output; no repair or
rounding occurs. Scalar exponent/bit/decimal limits and cumulative product/native
limits are retained. Each other product still uses its own original proof rules.

`author_step_model_torus.py --metre` authors the independent exact torus with an
explicit metre context. `author_step_independent_toroidal_units.py` combines it
with the retained mm cylinder and genuine OCCT metre sphere/box, changing only
reference IDs. Normalization produces torus radii 3000/1000 and center
(1000,2000,3000) in mm, with original circle/vertex IDs preserved. Independent
OCCT accepts the original and generated four-product files with equal physical
volume `24e9+12*pi+32e9*pi/3+6e9*pi²` mm³ and area
`52e6+(16e6+20)*pi+12e6*pi²` mm². Use the existing round-unit comparator binary
through `PLAINSOLID_OCCT_INDEPENDENT_TOROIDAL_UNITS`.

Tests cover native reload, exact reverse conversion of the standalone metre
torus, rejected nonbinary mm→m torus radii, wrong major/minor and seam radii,
underflow exponents, rounded vendor graph rejection, old API/tag rejection,
budgets, cancellation, race and bounded fuzz. Native torus evaluation remains the
previously qualified analytic representation, not an exact rational patch claim.
