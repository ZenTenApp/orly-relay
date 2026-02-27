# GLV Endomorphism + Strauss/wNAF Optimization for secp256k1

## What Is an Endomorphism?

An **endomorphism** is a map from a mathematical structure back to itself that
preserves the structure's operations. The word comes from Greek: *endo-*
("within") and *morphism* ("shape/form"). In the context of elliptic curves:

An endomorphism is a function `phi: E -> E` that maps points on the curve to
other points on the same curve, and that respects the group law:

```
phi(P + Q) = phi(P) + phi(Q)
```

for all points P, Q on E. This means the map is a **group homomorphism** from
the curve's point group to itself.

Every elliptic curve has two trivial endomorphisms: the identity map (`P -> P`)
and the negation map (`P -> -P`). Most curves over prime fields have *only*
these. But certain curves have additional, non-trivial endomorphisms -- and
secp256k1 is one of them.

### Why secp256k1 Has a Non-Trivial Endomorphism

The curve equation `y^2 = x^3 + 7` has no `x` term (the `a` coefficient is
zero). This means the right-hand side `x^3 + 7` has a **cube symmetry**: if you
replace `x` with `B*x` where `B` is a cube root of unity (`B^3 = 1`), then:

```
(B*x)^3 + 7 = B^3 * x^3 + 7 = 1 * x^3 + 7 = x^3 + 7
```

So the curve equation is unchanged. The map `phi(x, y) = (B*x, y)` sends a
point on the curve to another point on the curve. You can verify it also
respects point addition (this follows from the addition formulas depending only
on the curve equation, which is invariant under this substitution). Therefore
`phi` is an endomorphism.

This endomorphism has a corresponding **eigenvalue** in the scalar field: there
exists a scalar `L` such that `phi(P) = L * P` for all points P of prime order
n. This `L` is also a cube root of unity, but in F_n (the integers mod the curve
order) rather than F_p (the integers mod the field prime).

### Not All Curves Have This

A curve like secp256r1 (`y^2 = x^3 - 3x + b`) has a non-zero `a` coefficient,
so the cube substitution trick breaks:

```
(B*x)^3 - 3*(B*x) + b = x^3 + 7 - 3*B*x + b  ≠  x^3 - 3x + b
```

The extra `-3*B*x` term ruins the symmetry. This is why the GLV endomorphism
optimization only works on curves with `a = 0` (or certain other special
structures). secp256k1 was specifically chosen with this property.

## The Core Insight: A "Free" Scalar Multiplication

The secp256k1 curve is `y^2 = x^3 + 7` over a prime field F_p. Because the `a`
coefficient is zero, this curve has a special property: there exists a **cube
root of unity** in both the field and the scalar group. Concretely:

- **Beta** (`B`): a value in F_p where `B^3 = 1 mod p`, `B != 1`
- **Lambda** (`L`): a value in F_n (the scalar field) where `L^3 = 1 mod n`,
  `L != 1`

The relationship is: for any point `P = (x, y)` on the curve, the point
`(B*x, y)` is also on the curve, and it equals `L*P`. That is:

```
L * (x, y) = (B*x, y)
```

This is the **endomorphism**. Computing `L*P` normally requires ~256 doublings
and additions. But with this trick, it costs **one field multiplication**
(multiply x by B). That's the foundation of everything.

### The Constants

```
Beta  = 0x7ae96a2b657c07106e64479eac3434e99cf0497512f58995c1396c28719501ee
Lambda = 0x5363ad4cc05c30e0a5261c028812645a122e22ea20816678df02967c1b23bd72
```

## Stage 1: Scalar Decomposition (The GLV Trick)

Given a 256-bit scalar `k` and a point `P`, we want `k*P`. The GLV method
rewrites:

```
k*P = k1*P + k2*(L*P) = k1*P + k2*(B*P.x, P.y)
```

where `k1` and `k2` are each ~128 bits, satisfying `k = k1 + k2*L mod n`.

### How to find k1, k2

This is a **closest vector problem** on a 2D lattice. The lattice is the kernel
of the map `f(i,j) = i + L*j mod n` -- i.e., all pairs `(i,j)` where
`i + L*j = 0 mod n`. Two short basis vectors `beta` and `gamma` of this lattice
are precomputed (using extended Euclidean / Gauss lattice reduction). Then:

1. Compute `t1 = k * gamma_2 / det` and `t2 = -k * beta_2 / det` (where `det`
   is the lattice determinant)
2. Round to nearest integers: `t1_r = round(t1)`, `t2_r = round(t2)`
3. Compute `(k1, k2) = (k, 0) - t1_r * beta - t2_r * gamma`

The result: `|k1|, |k2| < ~sqrt(n)`, so each is roughly 128 bits.

### How libsecp256k1 Does It Without Division

libsecp256k1 precomputes the constants `g1`, `g2`, `minus_b1`, `minus_b2` to
perform this decomposition using only fixed-point integer arithmetic (no
division), via the identity:

```
round(k * g1 / 2^384)  ≈  k * b2 / n
round(k * g2 / 2^384)  ≈  k * (-b1) / n
```

The precomputed decomposition constants are:

```
g1       = 0x3086d221a7d46bcde86c90e49284eb153daa8a1471e8ca7fe893209a45dbb031
g2       = 0xe4437ed6010e88286f547fa90abfe4c4221208ac9df506c61571b4ae8ac47f71
minus_b1 = 0xe4437ed6010e88286f547fa90abfe4c3
minus_b2 = 0xfffffffffffffffe8a280ac50774346dd765cda83db1562c
```

The decomposition algorithm in `secp256k1_scalar_split_lambda()`:

```
c1 = round(k * g1 / 2^384)
c2 = round(k * g2 / 2^384)
c1 = c1 * (-b1) mod n
c2 = c2 * (-b2) mod n
k2 = c1 + c2 mod n
k1 = k - k2 * lambda mod n
```

Both `k1` and `k2` are guaranteed to be less than 2^128 in absolute value.

## Stage 2: wNAF Encoding (Sparse Signed Representation)

Standard binary representation of a 128-bit number has ~64 ones on average. The
**windowed Non-Adjacent Form** (wNAF) with window `w` represents the scalar as
a sequence of signed odd digits in `{-(2^(w-1)-1), ..., -1, 0, 1, ...,
2^(w-1)-1}` with the property that between any two non-zero digits there are at
least `w-1` zeros.

For `w = 5` (libsecp256k1's default for arbitrary points), the digits are in
`{-15, -13, ..., -1, 0, 1, ..., 15}` and on average only `128 / (5+1) ≈ 21` of
the 128 positions are non-zero. This means ~21 point additions instead of ~64 --
a huge reduction.

### The Tradeoff: Precomputed Tables

You need a precomputed table of odd multiples:

```
{1*P, 3*P, 5*P, ..., 15*P}    (8 entries for w=5)
```

For the endomorphism variant, you also need
`{1*L*P, 3*L*P, ..., 15*L*P}`, but these are essentially free -- just multiply
each x-coordinate by Beta.

### wNAF Conversion Algorithm

```
Input: scalar a, window width w
Output: wNAF array, bit count

1. If scalar is negative: negate it, set sign = -1
2. For each bit position from 0 to 127:
   a. If current bit matches carry: skip (digit = 0)
   b. Extract w bits starting at position: word = bits[pos:pos+w] + carry
   c. If MSB of word set: carry = 1, else carry = 0
   d. Apply carry: word = word - (carry << w)
   e. wNAF[pos] = sign * word
   f. Jump by w positions (next w-1 positions are guaranteed zero)
```

**Properties guaranteed:**
- Each coefficient is 0 or an odd integer in `[-(2^(w-1)-1), 2^(w-1)-1]`
- Non-zero entries separated by at least `w-1` zeros
- Representation is unique

## Stage 3: Strauss's Algorithm (Shared Doublings)

Computing `k1*P + k2*Q` naively means two independent double-and-add loops.
**Strauss's algorithm** (also called Shamir's trick) processes both scalars
simultaneously in a single loop:

```
result = point_at_infinity
for bit = highest_bit downto 0:
    result = 2 * result                          // one doubling (shared!)
    if wNAF_k1[bit] != 0:
        result += table_P[wNAF_k1[bit]]          // add from P's table
    if wNAF_k2[bit] != 0:
        result += table_Q[wNAF_k2[bit]]          // add from Q's table (lambda)
```

The key: **the doublings are shared**. Instead of ~128 doublings for k1 and ~128
doublings for k2 (total 256), you do only ~128 doublings total, with additions
interleaved from both scalars. Combined with wNAF sparsity, the total operation
count is roughly:

- **128 doublings** (one per bit of the ~128-bit scalars)
- **~42 additions** (~21 from k1's wNAF + ~21 from k2's wNAF)

Compare to naive binary method on the original 256-bit scalar: **256 doublings +
~128 additions**. That's roughly a **2x speedup** from the GLV split plus
additional gains from wNAF.

## Stage 4: Generator Optimization

For signature verification, libsecp256k1 computes `na*A + ng*G` where `G` is
the fixed generator. Since `G` is known at compile time, its multiples are
**precomputed into massive tables** (`secp256k1_pre_g` and
`secp256k1_pre_g_128`). The generator scalar `ng` is split into two 128-bit
halves via simple bit shifting (not GLV), and each half indexes into the
precomputed tables with window size 15. This eliminates all doublings for the
generator component.

## Summary of Speedup Sources

| Technique | What it saves | Approximate gain |
|-----------|--------------|-----------------|
| GLV endomorphism | Halves scalar length (256->128 bits) | ~2x fewer doublings |
| wNAF encoding | Sparser digit representation | ~3x fewer additions |
| Strauss interleaving | Shared doublings for multi-scalar | Eliminates redundant doublings |
| Precomputed generator tables | No computation for G multiples | Eliminates ~128 doublings for G |

## Complete Algorithm Flow

```
Input: scalar k, point P

[Stage 1: GLV Decomposition]
k -> split_lambda() -> (k1, k2) where k = k1 + L*k2 (mod n)
    |k1|, |k2| < 2^128

[Stage 2: Precomputation]
P -> odd_multiples_table() -> [P, 3P, 5P, ..., 15P]
L(P) = (B*P.x, P.y)        -> [L*P, 3*L*P, ..., 15*L*P]  (just multiply x by B)

[Stage 3: wNAF Encoding]
k1 -> ecmult_wnaf(w=5) -> wnaf_k1[]   (~21 non-zero entries out of 128)
k2 -> ecmult_wnaf(w=5) -> wnaf_k2[]   (~21 non-zero entries out of 128)

[Stage 4: Strauss Accumulation]
result = infinity
for bit = 127 downto 0:
    result = 2 * result
    if wnaf_k1[bit] != 0: result += table_P[wnaf_k1[bit]]
    if wnaf_k2[bit] != 0: result += table_LP[wnaf_k2[bit]]

Output: k*P
```

## libsecp256k1 Source Code Map

| Phase | File | Function | Key Variables |
|-------|------|----------|---------------|
| Decompose | `scalar_impl.h` | `secp256k1_scalar_split_lambda()` | `g1`, `g2`, `minus_b1`, `minus_b2`, `lambda` |
| Endomorphism | `group_impl.h` | `secp256k1_ge_mul_lambda()` | `beta` |
| Precompute | `ecmult_impl.h` | `secp256k1_ecmult_odd_multiples_table()` | `pre_a[]`, `zr[]` |
| wNAF encode | `ecmult_impl.h` | `secp256k1_ecmult_wnaf()` | `wnaf[]`, carry, sign |
| Accumulate | `ecmult_impl.h` | `secp256k1_ecmult_strauss_wnaf()` | `r`, `bits` |
| Table lookup | `ecmult_impl.h` | `secp256k1_ecmult_table_get_ge()` | `n` (wNAF coeff) |
| Lambda lookup | `ecmult_impl.h` | `secp256k1_ecmult_table_get_ge_lambda()` | `n`, `beta*x` cache |

## References

### Original Papers

- Gallant, Lambert, Vanstone -- "Faster Point Multiplication on Elliptic Curves
  with Efficient Endomorphisms" (CRYPTO 2001)
- Galbraith, Lin, Scott -- "Endomorphisms for Faster Elliptic Curve Cryptography
  on a Large Class of Curves" (EUROCRYPT 2009)
- Sica, Ciet, Quisquater -- "Analysis of the GLV Method" (SAC 2002)
- Brumley -- "Faster software for fast endomorphisms" (COSADE 2015)

### Practical Explanations

- https://paulmillr.com/posts/noble-secp256k1-fast-ecc/
- https://hackmd.io/@davidnevadoc/BJRAS6Opi
- https://medium.com/@francomangone18/elliptic-curves-in-depth-part-10-aedd493d8170
