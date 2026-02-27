//go:build !js && !wasm && !tinygo && !wasm32

package p256k1

import (
	"testing"
)

var benchFieldA = FieldElement{
	n:          [5]uint64{0x4567890abcdef, 0xcba9876543210, 0x3456789abcdef, 0xcba0987654321, 0x123456789ab},
	magnitude:  1,
	normalized: true,
}

var benchFieldB = FieldElement{
	n:          [5]uint64{0xdef1234567890, 0x6543210fedcba, 0xcba1234567890, 0x7654321abcdef, 0xfedcba98765},
	magnitude:  1,
	normalized: true,
}

// BenchmarkFieldMulAsm benchmarks the assembly field multiplication
func BenchmarkFieldMulAsm(b *testing.B) {
	if !hasFieldAsm() {
		b.Skip("Assembly not available")
	}

	var r FieldElement
	for i := 0; i < b.N; i++ {
		fieldMulAsm(&r, &benchFieldA, &benchFieldB)
	}
}

// BenchmarkFieldMulPureGo benchmarks the pure Go field multiplication
func BenchmarkFieldMulPureGo(b *testing.B) {
	var r FieldElement
	for i := 0; i < b.N; i++ {
		fieldMulPureGo(&r, &benchFieldA, &benchFieldB)
	}
}

// BenchmarkFieldSqrAsm benchmarks the assembly field squaring
func BenchmarkFieldSqrAsm(b *testing.B) {
	if !hasFieldAsm() {
		b.Skip("Assembly not available")
	}

	var r FieldElement
	for i := 0; i < b.N; i++ {
		fieldSqrAsm(&r, &benchFieldA)
	}
}

// BenchmarkFieldSqrPureGo benchmarks the pure Go field squaring (via mul)
func BenchmarkFieldSqrPureGo(b *testing.B) {
	var r FieldElement
	for i := 0; i < b.N; i++ {
		fieldMulPureGo(&r, &benchFieldA, &benchFieldA)
	}
}

// BenchmarkFieldMul benchmarks the full mul method (which uses assembly when available)
func BenchmarkFieldMul(b *testing.B) {
	r := new(FieldElement)
	a := benchFieldA
	bb := benchFieldB
	for i := 0; i < b.N; i++ {
		r.mul(&a, &bb)
	}
}

// BenchmarkFieldSqr benchmarks the full sqr method (which uses assembly when available)
func BenchmarkFieldSqr(b *testing.B) {
	r := new(FieldElement)
	a := benchFieldA
	for i := 0; i < b.N; i++ {
		r.sqr(&a)
	}
}

// BMI2 benchmarks

// BenchmarkFieldMulAsmBMI2 benchmarks the BMI2 assembly field multiplication
func BenchmarkFieldMulAsmBMI2(b *testing.B) {
	if !hasFieldAsmBMI2() {
		b.Skip("BMI2+ADX assembly not available")
	}

	var r FieldElement
	for i := 0; i < b.N; i++ {
		fieldMulAsmBMI2(&r, &benchFieldA, &benchFieldB)
	}
}

// BenchmarkFieldSqrAsmBMI2 benchmarks the BMI2 assembly field squaring
func BenchmarkFieldSqrAsmBMI2(b *testing.B) {
	if !hasFieldAsmBMI2() {
		b.Skip("BMI2+ADX assembly not available")
	}

	var r FieldElement
	for i := 0; i < b.N; i++ {
		fieldSqrAsmBMI2(&r, &benchFieldA)
	}
}
