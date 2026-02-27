package avx

import "testing"

func TestMulInt(t *testing.T) {
	// Test 3 * X = X + X + X
	var x, tripleX, addX FieldElement
	x.N[0].Lo = 12345
	
	tripleX.MulInt(&x, 3)
	addX.Add(&x, &x)
	addX.Add(&addX, &x)
	
	if !tripleX.Equal(&addX) {
		t.Errorf("3*X != X+X+X: MulInt=%+v, Add=%+v", tripleX, addX)
	}
	
	// Test 2 * Y = Y + Y
	var y, doubleY, addY FieldElement
	y.N[0].Lo = 0xFFFFFFFFFFFFFFFF
	y.N[0].Hi = 0xFFFFFFFFFFFFFFFF
	
	doubleY.MulInt(&y, 2)
	addY.Add(&y, &y)
	
	if !doubleY.Equal(&addY) {
		t.Errorf("2*Y != Y+Y: MulInt=%+v, Add=%+v", doubleY, addY)
	}
}
