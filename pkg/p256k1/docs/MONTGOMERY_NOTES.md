# Montgomery Multiplication Implementation Notes

## Status
Montgomery multiplication has been partially implemented in `field.go`. The current implementation provides the API structure but uses standard multiplication internally.

## Current Implementation
- `ToMontgomery()`: Converts to Montgomery form using R² multiplication
- `FromMontgomery()`: Converts from Montgomery form (currently uses standard multiplication)
- `MontgomeryMul()`: Multiplies two Montgomery-form elements (currently uses standard multiplication)
- `montgomeryReduce()`: REDC algorithm implementation (partially complete)

## Issues
1. The `FromMontgomery()` implementation needs proper R⁻¹ computation
2. The `MontgomeryMul()` should use the REDC algorithm directly instead of standard multiplication
3. The R² constant may need verification
4. Tests are currently failing due to incomplete implementation

## Next Steps
1. Compute R⁻¹ mod p correctly
2. Implement proper REDC algorithm in MontgomeryMul
3. Verify R² constant against reference implementation
4. Add comprehensive tests

## References
- Montgomery reduction: https://en.wikipedia.org/wiki/Montgomery_modular_multiplication
- secp256k1 field implementation: src/field_5x52.h

