package values

import (
	"time"
)

// ToUintPointer returns a pointer to the uint value passed in.
func ToUintPointer(v uint) *uint {
	return &v
}

// ToIntPointer returns a pointer to the int value passed in.
func ToIntPointer(v int) *int {
	return &v
}

// ToUint8Pointer returns a pointer to the uint8 value passed in.
func ToUint8Pointer(v uint8) *uint8 {
	return &v
}

// ToUint16Pointer returns a pointer to the uint16 value passed in.
func ToUint16Pointer(v uint16) *uint16 {
	return &v
}

// ToUint32Pointer returns a pointer to the uint32 value passed in.
func ToUint32Pointer(v uint32) *uint32 {
	return &v
}

// ToUint64Pointer returns a pointer to the uint64 value passed in.
func ToUint64Pointer(v uint64) *uint64 {
	return &v
}

// ToInt8Pointer returns a pointer to the int8 value passed in.
func ToInt8Pointer(v int8) *int8 {
	return &v
}

// ToInt16Pointer returns a pointer to the int16 value passed in.
func ToInt16Pointer(v int16) *int16 {
	return &v
}

// ToInt32Pointer returns a pointer to the int32 value passed in.
func ToInt32Pointer(v int32) *int32 {
	return &v
}

// ToInt64Pointer returns a pointer to the int64 value passed in.
func ToInt64Pointer(v int64) *int64 {
	return &v
}

// ToFloat32Pointer returns a pointer to the float32 value passed in.
func ToFloat32Pointer(v float32) *float32 {
	return &v
}

// ToFloat64Pointer returns a pointer to the float64 value passed in.
func ToFloat64Pointer(v float64) *float64 {
	return &v
}

// ToStringPointer returns a pointer to the string value passed in.
func ToStringPointer(v string) *string {
	return &v
}

// ToStringSlicePointer returns a pointer to the []string value passed in.
func ToStringSlicePointer(v []string) *[]string {
	return &v
}

// ToTimePointer returns a pointer to the time.Time value passed in.
func ToTimePointer(v time.Time) *time.Time {
	return &v
}

// ToDurationPointer returns a pointer to the time.Duration value passed in.
func ToDurationPointer(v time.Duration) *time.Duration {
	return &v
}

// ToBytesPointer returns a pointer to the []byte value passed in.
func ToBytesPointer(v []byte) *[]byte {
	return &v
}

// ToByteSlicesPointer returns a pointer to the [][]byte value passed in.
func ToByteSlicesPointer(v [][]byte) *[][]byte {
	return &v
}
