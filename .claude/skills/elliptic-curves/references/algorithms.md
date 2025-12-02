# Elliptic Curve Algorithms

Detailed pseudocode for core elliptic curve operations.

## Field Arithmetic

### Modular Addition

```
function mod_add(a, b, p):
    result = a + b
    if result >= p:
        result = result - p
    return result
```

### Modular Subtraction

```
function mod_sub(a, b, p):
    if a >= b:
        return a - b
    else:
        return p - b + a
```

### Modular Multiplication

For general case:
```
function mod_mul(a, b, p):
    return (a * b) mod p
```

For secp256k1 optimized (Barrett reduction):
```
function mod_mul_secp256k1(a, b):
    # Compute full 512-bit product
    product = a * b

    # Split into high and low 256-bit parts
    low = product & ((1 << 256) - 1)
    high = product >> 256

    # Reduce: result ≡ low + high * (2³² + 977) (mod p)
    result = low + high * (1 << 32) + high * 977

    # May need additional reduction
    while result >= p:
        result = result - p

    return result
```

### Modular Inverse

**Extended Euclidean Algorithm**:
```
function mod_inverse(a, p):
    if a == 0:
        error "No inverse exists for 0"

    old_r, r = p, a
    old_s, s = 0, 1

    while r != 0:
        quotient = old_r / r
        old_r, r = r, old_r - quotient * r
        old_s, s = s, old_s - quotient * s

    if old_r != 1:
        error "No inverse exists"

    if old_s < 0:
        old_s = old_s + p

    return old_s
```

**Fermat's Little Theorem** (for prime p):
```
function mod_inverse_fermat(a, p):
    return mod_exp(a, p - 2, p)
```

### Modular Exponentiation (Square-and-Multiply)

```
function mod_exp(base, exp, p):
    result = 1
    base = base mod p

    while exp > 0:
        if exp & 1:  # exp is odd
            result = (result * base) mod p
        exp = exp >> 1
        base = (base * base) mod p

    return result
```

### Modular Square Root (Tonelli-Shanks)

For secp256k1 where p ≡ 3 (mod 4):
```
function mod_sqrt(a, p):
    # For p ≡ 3 (mod 4), sqrt(a) = a^((p+1)/4)
    return mod_exp(a, (p + 1) / 4, p)
```

## Point Operations

### Point Validation

```
function is_on_curve(P, a, b, p):
    if P is infinity:
        return true

    x, y = P
    left = (y * y) mod p
    right = (x * x * x + a * x + b) mod p

    return left == right
```

### Point Addition (Affine Coordinates)

```
function point_add(P, Q, a, p):
    if P is infinity:
        return Q
    if Q is infinity:
        return P

    x1, y1 = P
    x2, y2 = Q

    if x1 == x2:
        if y1 == mod_neg(y2, p):  # P = -Q
            return infinity
        else:  # P == Q
            return point_double(P, a, p)

    # λ = (y2 - y1) / (x2 - x1)
    numerator = mod_sub(y2, y1, p)
    denominator = mod_sub(x2, x1, p)
    λ = mod_mul(numerator, mod_inverse(denominator, p), p)

    # x3 = λ² - x1 - x2
    x3 = mod_sub(mod_sub(mod_mul(λ, λ, p), x1, p), x2, p)

    # y3 = λ(x1 - x3) - y1
    y3 = mod_sub(mod_mul(λ, mod_sub(x1, x3, p), p), y1, p)

    return (x3, y3)
```

### Point Doubling (Affine Coordinates)

```
function point_double(P, a, p):
    if P is infinity:
        return infinity

    x, y = P

    if y == 0:
        return infinity

    # λ = (3x² + a) / (2y)
    numerator = mod_add(mod_mul(3, mod_mul(x, x, p), p), a, p)
    denominator = mod_mul(2, y, p)
    λ = mod_mul(numerator, mod_inverse(denominator, p), p)

    # x3 = λ² - 2x
    x3 = mod_sub(mod_mul(λ, λ, p), mod_mul(2, x, p), p)

    # y3 = λ(x - x3) - y
    y3 = mod_sub(mod_mul(λ, mod_sub(x, x3, p), p), y, p)

    return (x3, y3)
```

### Point Negation

```
function point_negate(P, p):
    if P is infinity:
        return infinity

    x, y = P
    return (x, p - y)
```

## Scalar Multiplication

### Double-and-Add (Left-to-Right)

```
function scalar_mult_double_add(k, P, a, p):
    if k == 0 or P is infinity:
        return infinity

    if k < 0:
        k = -k
        P = point_negate(P, p)

    R = infinity
    bits = binary_representation(k)  # MSB first

    for bit in bits:
        R = point_double(R, a, p)
        if bit == 1:
            R = point_add(R, P, a, p)

    return R
```

### Montgomery Ladder (Constant-Time)

```
function scalar_mult_montgomery(k, P, a, p):
    R0 = infinity
    R1 = P

    bits = binary_representation(k)  # MSB first

    for bit in bits:
        if bit == 0:
            R1 = point_add(R0, R1, a, p)
            R0 = point_double(R0, a, p)
        else:
            R0 = point_add(R0, R1, a, p)
            R1 = point_double(R1, a, p)

    return R0
```

### w-NAF Scalar Multiplication

```
function compute_wNAF(k, w):
    # Convert scalar to width-w Non-Adjacent Form
    naf = []

    while k > 0:
        if k & 1:  # k is odd
            # Get w-bit window
            digit = k mod (1 << w)
            if digit >= (1 << (w-1)):
                digit = digit - (1 << w)
            naf.append(digit)
            k = k - digit
        else:
            naf.append(0)
        k = k >> 1

    return naf

function scalar_mult_wNAF(k, P, w, a, p):
    # Precompute odd multiples: [P, 3P, 5P, ..., (2^(w-1)-1)P]
    precomp = [P]
    P2 = point_double(P, a, p)
    for i in range(1, 1 << (w-1)):
        precomp.append(point_add(precomp[-1], P2, a, p))

    # Convert k to w-NAF
    naf = compute_wNAF(k, w)

    # Compute scalar multiplication
    R = infinity
    for i in range(len(naf) - 1, -1, -1):
        R = point_double(R, a, p)
        digit = naf[i]
        if digit > 0:
            R = point_add(R, precomp[(digit - 1) / 2], a, p)
        elif digit < 0:
            R = point_add(R, point_negate(precomp[(-digit - 1) / 2], p), a, p)

    return R
```

### Shamir's Trick (Multi-Scalar)

For computing k₁P + k₂Q efficiently:

```
function multi_scalar_mult(k1, P, k2, Q, a, p):
    # Precompute P + Q
    PQ = point_add(P, Q, a, p)

    # Get binary representations (same length, padded)
    bits1 = binary_representation(k1)
    bits2 = binary_representation(k2)
    max_len = max(len(bits1), len(bits2))
    bits1 = pad_left(bits1, max_len)
    bits2 = pad_left(bits2, max_len)

    R = infinity

    for i in range(max_len):
        R = point_double(R, a, p)

        b1, b2 = bits1[i], bits2[i]

        if b1 == 1 and b2 == 1:
            R = point_add(R, PQ, a, p)
        elif b1 == 1:
            R = point_add(R, P, a, p)
        elif b2 == 1:
            R = point_add(R, Q, a, p)

    return R
```

## Jacobian Coordinates

More efficient for repeated operations.

### Conversion

```
# Affine to Jacobian
function affine_to_jacobian(P):
    if P is infinity:
        return (1, 1, 0)  # Jacobian infinity
    x, y = P
    return (x, y, 1)

# Jacobian to Affine
function jacobian_to_affine(P, p):
    X, Y, Z = P
    if Z == 0:
        return infinity

    Z_inv = mod_inverse(Z, p)
    Z_inv2 = mod_mul(Z_inv, Z_inv, p)
    Z_inv3 = mod_mul(Z_inv2, Z_inv, p)

    x = mod_mul(X, Z_inv2, p)
    y = mod_mul(Y, Z_inv3, p)

    return (x, y)
```

### Point Doubling (Jacobian)

For curve y² = x³ + 7 (a = 0):

```
function jacobian_double(P, p):
    X, Y, Z = P

    if Y == 0:
        return (1, 1, 0)  # infinity

    # For a = 0: M = 3*X²
    S = mod_mul(4, mod_mul(X, mod_mul(Y, Y, p), p), p)
    M = mod_mul(3, mod_mul(X, X, p), p)

    X3 = mod_sub(mod_mul(M, M, p), mod_mul(2, S, p), p)
    Y3 = mod_sub(mod_mul(M, mod_sub(S, X3, p), p),
                  mod_mul(8, mod_mul(Y, Y, mod_mul(Y, Y, p), p), p), p)
    Z3 = mod_mul(2, mod_mul(Y, Z, p), p)

    return (X3, Y3, Z3)
```

### Point Addition (Jacobian + Affine)

Mixed addition is faster when one point is in affine:

```
function jacobian_add_affine(P, Q, p):
    # P in Jacobian (X1, Y1, Z1), Q in affine (x2, y2)
    X1, Y1, Z1 = P
    x2, y2 = Q

    if Z1 == 0:
        return affine_to_jacobian(Q)

    Z1Z1 = mod_mul(Z1, Z1, p)
    U2 = mod_mul(x2, Z1Z1, p)
    S2 = mod_mul(y2, mod_mul(Z1, Z1Z1, p), p)

    H = mod_sub(U2, X1, p)
    HH = mod_mul(H, H, p)
    I = mod_mul(4, HH, p)
    J = mod_mul(H, I, p)
    r = mod_mul(2, mod_sub(S2, Y1, p), p)
    V = mod_mul(X1, I, p)

    X3 = mod_sub(mod_sub(mod_mul(r, r, p), J, p), mod_mul(2, V, p), p)
    Y3 = mod_sub(mod_mul(r, mod_sub(V, X3, p), p), mod_mul(2, mod_mul(Y1, J, p), p), p)
    Z3 = mod_mul(mod_sub(mod_mul(mod_add(Z1, H, p), mod_add(Z1, H, p), p),
                          mod_add(Z1Z1, HH, p), p), 1, p)

    return (X3, Y3, Z3)
```

## GLV Endomorphism (secp256k1)

### Scalar Decomposition

```
# Constants for secp256k1
LAMBDA = 0x5363AD4CC05C30E0A5261C028812645A122E22EA20816678DF02967C1B23BD72
BETA = 0x7AE96A2B657C07106E64479EAC3434E99CF0497512F58995C1396C28719501EE

# Decomposition coefficients
A1 = 0x3086D221A7D46BCDE86C90E49284EB15
B1 = 0x114CA50F7A8E2F3F657C1108D9D44CFD8
A2 = 0xE4437ED6010E88286F547FA90ABFE4C3
B2 = A1

function glv_decompose(k, n):
    # Compute c1 = round(b2 * k / n)
    # Compute c2 = round(-b1 * k / n)
    c1 = (B2 * k + n // 2) // n
    c2 = (-B1 * k + n // 2) // n

    # k1 = k - c1*A1 - c2*A2
    # k2 = -c1*B1 - c2*B2
    k1 = k - c1 * A1 - c2 * A2
    k2 = -c1 * B1 - c2 * B2

    return (k1, k2)

function glv_scalar_mult(k, P, p, n):
    k1, k2 = glv_decompose(k, n)

    # Compute endomorphism: φ(P) = (β*x, y)
    x, y = P
    phi_P = (mod_mul(BETA, x, p), y)

    # Use Shamir's trick: k1*P + k2*φ(P)
    return multi_scalar_mult(k1, P, k2, phi_P, 0, p)
```

## Batch Inversion

Amortize expensive inversions over multiple points:

```
function batch_invert(values, p):
    n = len(values)
    if n == 0:
        return []

    # Compute cumulative products
    products = [values[0]]
    for i in range(1, n):
        products.append(mod_mul(products[-1], values[i], p))

    # Invert the final product
    inv = mod_inverse(products[-1], p)

    # Compute individual inverses
    inverses = [0] * n
    for i in range(n - 1, 0, -1):
        inverses[i] = mod_mul(inv, products[i - 1], p)
        inv = mod_mul(inv, values[i], p)
    inverses[0] = inv

    return inverses
```

## Key Generation

```
function generate_keypair(G, n, p):
    # Generate random private key
    d = random_integer(1, n - 1)

    # Compute public key
    Q = scalar_mult(d, G)

    return (d, Q)
```

## Point Compression/Decompression

```
function compress_point(P, p):
    if P is infinity:
        return bytes([0x00])

    x, y = P
    prefix = 0x02 if (y % 2 == 0) else 0x03
    return bytes([prefix]) + x.to_bytes(32, 'big')

function decompress_point(compressed, a, b, p):
    prefix = compressed[0]

    if prefix == 0x00:
        return infinity

    x = int.from_bytes(compressed[1:], 'big')

    # Compute y² = x³ + ax + b
    y_squared = mod_add(mod_add(mod_mul(x, mod_mul(x, x, p), p),
                                  mod_mul(a, x, p), p), b, p)

    # Compute y = sqrt(y²)
    y = mod_sqrt(y_squared, p)

    # Select correct y based on prefix
    if (prefix == 0x02) != (y % 2 == 0):
        y = p - y

    return (x, y)
```