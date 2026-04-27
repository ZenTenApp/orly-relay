package main

// Minimal QR code encoder for npub strings.
// Generates version-4-M (33x33 modules, medium EC) alphanumeric QR codes
// and renders them as an SVG string with a center logo cutout.
//
// npub1... is 63 chars. Version 4-M alphanumeric capacity = 78 chars. Plenty.
// Medium EC (~15% correction) tolerates the center logo area.

// qrSVG generates a QR code SVG string for the given data.
// The SVG has a white background, black modules, and a centered logo placeholder.
func qrSVG(data string, size int, logoSVG string) string {
	mods := qrEncode(data)
	if mods == nil {
		return ""
	}
	n := len(mods)
	// Module size in SVG units.
	quiet := 2 // quiet zone modules
	total := n + quiet*2
	modSize := size / total
	if modSize < 1 {
		modSize = 1
	}
	svgSize := total * modSize

	svg := "<svg xmlns='http://www.w3.org/2000/svg' width='" + itoa(svgSize) + "' height='" + itoa(svgSize) + "' viewBox='0 0 " + itoa(svgSize) + " " + itoa(svgSize) + "'>"
	svg += "<rect width='100%' height='100%' fill='white'/>"

	// Center logo area (mask out modules).
	logoMods := 9 // 9x9 module area in center
	logoStart := (n - logoMods) / 2
	logoEnd := logoStart + logoMods

	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			if mods[y][x]&1 == 1 {
				// Skip modules in logo area.
				if x >= logoStart && x < logoEnd && y >= logoStart && y < logoEnd {
					continue
				}
				px := (x + quiet) * modSize
				py := (y + quiet) * modSize
				svg += "<rect x='" + itoa(px) + "' y='" + itoa(py) + "' width='" + itoa(modSize) + "' height='" + itoa(modSize) + "' fill='black'/>"
			}
		}
	}

	// Center logo.
	if logoSVG != "" {
		logoPixStart := (logoStart + quiet) * modSize
		logoPixSize := logoMods * modSize
		svg += "<g transform='translate(" + itoa(logoPixStart) + "," + itoa(logoPixStart) + ")'>"
		svg += "<rect width='" + itoa(logoPixSize) + "' height='" + itoa(logoPixSize) + "' fill='white' rx='4'/>"
		pad := logoPixSize / 6
		inner := logoPixSize - pad*2
		svg += "<g transform='translate(" + itoa(pad) + "," + itoa(pad) + ")'>"
		// Inject logo SVG, rewriting width/height.
		svg += "<svg width='" + itoa(inner) + "' height='" + itoa(inner) + "' viewBox='0 0 100 100'>"
		// Strip outer <svg> tags from logoSVG and inject content.
		svg += extractSVGContent(logoSVG)
		svg += "</svg></g></g>"
	}

	svg += "</svg>"
	return svg
}

// extractSVGContent strips the outer <svg ...> and </svg> tags, returning inner content.
func extractSVGContent(s string) string {
	// Find end of opening <svg ...> tag.
	i := 0
	for i < len(s) && s[i] != '<' {
		i++
	}
	if i >= len(s) {
		return s
	}
	// Find closing > of opening tag.
	j := i + 1
	for j < len(s) && s[j] != '>' {
		j++
	}
	if j >= len(s) {
		return s
	}
	start := j + 1
	// Find </svg>.
	end := len(s)
	for k := len(s) - 1; k >= 6; k-- {
		if s[k] == '>' && k >= 5 && s[k-5:k+1] == "</svg>" {
			end = k - 5
			break
		}
	}
	return s[start:end]
}

// --- QR encoder: Version 4-M, alphanumeric mode ---

const qrSize = 33 // Version 4 = 33x33

// qrEncode produces the module matrix for a version-4-M alphanumeric QR code.
func qrEncode(data string) [][]byte {
	gfInit()
	// Encode data bits.
	bits := qrAlphanumericBits(data)
	if bits == nil {
		return nil
	}
	// Add terminator + padding.
	bits = qrPadBits(bits)
	// Compute error correction.
	codewords := bitsToBytes(bits)
	ecWords := rsEncode(codewords)
	// Interleave (single block for v4-M).
	var allBits []byte
	for _, b := range codewords {
		allBits = append(allBits, byteToBits(b)...)
	}
	for _, b := range ecWords {
		allBits = append(allBits, byteToBits(b)...)
	}
	// Remainder bits for version 4: 7.
	for i := 0; i < 7; i++ {
		allBits = append(allBits, 0)
	}

	// Place modules.
	mods := make([][]byte, qrSize)
	for i := range mods {
		mods[i] = make([]byte, qrSize)
	}
	// reserved = 2 means "function pattern, don't overwrite"
	qrPlaceFinderPatterns(mods)
	qrPlaceAlignmentPattern(mods)
	qrPlaceTimingPatterns(mods)
	qrPlaceDarkModule(mods)
	qrReserveFormatArea(mods)
	qrReserveVersionArea(mods) // no-op for v4
	qrPlaceData(mods, allBits)
	qrApplyBestMask(mods)
	return mods
}

// Alphanumeric character set.
const alphaChars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ $%*+-./:"

func alphaVal(c byte) int {
	for i := 0; i < len(alphaChars); i++ {
		if alphaChars[i] == c {
			return i
		}
	}
	return -1
}

func qrAlphanumericBits(data string) []byte {
	udata := toUpper(data)
	// Mode indicator: 0010 (alphanumeric).
	bits := []byte{0, 0, 1, 0}
	// Character count: 9 bits for version 4.
	count := len(udata)
	for i := 8; i >= 0; i-- {
		bits = append(bits, byte((count>>uint(i))&1))
	}
	// Encode pairs.
	for i := 0; i+1 < len(udata); i += 2 {
		v1 := alphaVal(udata[i])
		v2 := alphaVal(udata[i+1])
		if v1 < 0 || v2 < 0 {
			return nil
		}
		val := v1*45 + v2
		for b := 10; b >= 0; b-- {
			bits = append(bits, byte((val>>uint(b))&1))
		}
	}
	if len(udata)%2 == 1 {
		v := alphaVal(udata[len(udata)-1])
		if v < 0 {
			return nil
		}
		for b := 5; b >= 0; b-- {
			bits = append(bits, byte((v>>uint(b))&1))
		}
	}
	return bits
}

func toUpper(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		b[i] = c
	}
	return string(b)
}

// v4-M: 80 data codewords total.
const v4MDataCodewords = 80

func qrPadBits(bits []byte) []byte {
	totalBits := v4MDataCodewords * 8
	// Add terminator (up to 4 zeros).
	term := 4
	if totalBits-len(bits) < 4 {
		term = totalBits - len(bits)
	}
	for i := 0; i < term; i++ {
		bits = append(bits, 0)
	}
	// Pad to byte boundary.
	for len(bits)%8 != 0 {
		bits = append(bits, 0)
	}
	// Pad bytes.
	padBytes := []byte{0xEC, 0x11}
	idx := 0
	for len(bits) < totalBits {
		bits = append(bits, byteToBits(padBytes[idx%2])...)
		idx++
	}
	return bits[:totalBits]
}

func bitsToBytes(bits []byte) []byte {
	n := len(bits) / 8
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		var v byte
		for j := 0; j < 8; j++ {
			v = v<<1 | bits[i*8+j]
		}
		out[i] = v
	}
	return out
}

func byteToBits(b byte) []byte {
	bits := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		bits[7-i] = (b >> uint(i)) & 1
	}
	return bits
}

// --- Reed-Solomon for v4-M: 1 block, 80 data codewords, 18 EC codewords ---

const v4MECCodewords = 18

// GF(256) with primitive polynomial 0x11d.
var gfExp [512]byte
var gfLog [256]byte
var gfReady bool

func gfInit() {
	if gfReady {
		return
	}
	gfReady = true
	v := 1
	for i := 0; i < 255; i++ {
		gfExp[i] = byte(v)
		gfLog[v] = byte(i)
		v <<= 1
		if v >= 256 {
			v ^= 0x11d
		}
	}
	for i := 255; i < 512; i++ {
		gfExp[i] = gfExp[i-255]
	}
}

func gfMul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return gfExp[int(gfLog[a])+int(gfLog[b])]
}

func rsEncode(data []byte) []byte {
	// Generator polynomial for 18 EC codewords.
	gen := rsGeneratorPoly(v4MECCodewords)
	result := make([]byte, v4MECCodewords)
	for _, d := range data {
		factor := d ^ result[0]
		copy(result, result[1:])
		result[v4MECCodewords-1] = 0
		for i := 0; i < v4MECCodewords; i++ {
			result[i] ^= gfMul(gen[i], factor)
		}
	}
	return result
}

func rsGeneratorPoly(n int) []byte {
	poly := []byte{1}
	for i := 0; i < n; i++ {
		newPoly := make([]byte, len(poly)+1)
		for j := 0; j < len(poly); j++ {
			newPoly[j] ^= poly[j]
			newPoly[j+1] ^= gfMul(poly[j], gfExp[i])
		}
		poly = newPoly
	}
	return poly[1:] // drop leading 1
}

// --- Module placement ---

func setMod(mods [][]byte, x, y int, val byte) {
	if x >= 0 && x < qrSize && y >= 0 && y < qrSize {
		mods[y][x] = val
	}
}

func qrPlaceFinderPatterns(mods [][]byte) {
	place := func(cx, cy int) {
		for dy := -4; dy <= 4; dy++ {
			for dx := -4; dx <= 4; dx++ {
				x, y := cx+dx, cy+dy
				if x < 0 || x >= qrSize || y < 0 || y >= qrSize {
					continue
				}
				adx, ady := dx, dy
				if adx < 0 {
					adx = -adx
				}
				if ady < 0 {
					ady = -ady
				}
				mx := adx
				if ady > mx {
					mx = ady
				}
				var v byte
				switch {
				case mx == 4:
					v = 2 // separator (white, reserved)
				case mx == 0 || mx == 2:
					v = 3 // black, reserved
				case mx == 1:
					v = 2 // white, reserved
				case mx == 3:
					v = 3 // black, reserved
				}
				mods[y][x] = v
			}
		}
	}
	place(3, 3)              // top-left
	place(qrSize-4, 3)      // top-right
	place(3, qrSize-4)      // bottom-left
}

func qrPlaceAlignmentPattern(mods [][]byte) {
	// Version 4: alignment pattern at (26, 26).
	cx, cy := 26, 26
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			adx, ady := dx, dy
			if adx < 0 {
				adx = -adx
			}
			if ady < 0 {
				ady = -ady
			}
			mx := adx
			if ady > mx {
				mx = ady
			}
			var v byte
			if mx == 1 {
				v = 2 // white, reserved
			} else {
				v = 3 // black, reserved
			}
			mods[cy+dy][cx+dx] = v
		}
	}
}

func qrPlaceTimingPatterns(mods [][]byte) {
	for i := 8; i < qrSize-8; i++ {
		v := byte(3) // black reserved
		if i%2 == 1 {
			v = 2 // white reserved
		}
		if mods[6][i] == 0 {
			mods[6][i] = v
		}
		if mods[i][6] == 0 {
			mods[i][6] = v
		}
	}
}

func qrPlaceDarkModule(mods [][]byte) {
	mods[qrSize-8][8] = 3 // always dark
}

func qrReserveFormatArea(mods [][]byte) {
	// Around top-left finder.
	for i := 0; i < 9; i++ {
		if mods[8][i] == 0 {
			mods[8][i] = 2
		}
		if mods[i][8] == 0 {
			mods[i][8] = 2
		}
	}
	// Below top-right finder.
	for i := 0; i < 8; i++ {
		if mods[8][qrSize-1-i] == 0 {
			mods[8][qrSize-1-i] = 2
		}
	}
	// Right of bottom-left finder.
	for i := 0; i < 7; i++ {
		if mods[qrSize-1-i][8] == 0 {
			mods[qrSize-1-i][8] = 2
		}
	}
}

func qrReserveVersionArea(mods [][]byte) {
	// Version info blocks only for version >= 7. No-op for v4.
}

func qrPlaceData(mods [][]byte, bits []byte) {
	idx := 0
	// Data is placed in 2-column strips, right to left, bottom to top / top to bottom alternating.
	x := qrSize - 1
	upward := true
	for x > 0 {
		if x == 6 {
			x-- // skip timing column
		}
		for row := 0; row < qrSize; row++ {
			y := row
			if upward {
				y = qrSize - 1 - row
			}
			for dx := 0; dx <= 1; dx++ {
				cx := x - dx
				if mods[y][cx] != 0 {
					continue // reserved
				}
				if idx < len(bits) {
					mods[y][cx] = bits[idx] // 0 or 1
				}
				idx++
			}
		}
		upward = !upward
		x -= 2
	}
}

// --- Masking ---

// v4-M format info strings (pre-computed for masks 0-7).
// Format: error correction level M (01) + mask pattern + BCH error correction.
var formatBits = [8][15]byte{
	{1, 0, 1, 0, 1, 0, 0, 0, 0, 0, 1, 0, 0, 1, 0}, // mask 0
	{1, 0, 1, 0, 0, 0, 1, 0, 1, 1, 0, 1, 1, 0, 0}, // mask 1
	{1, 0, 1, 1, 1, 1, 0, 1, 1, 0, 0, 0, 1, 1, 1}, // mask 2
	{1, 0, 1, 1, 0, 1, 1, 1, 0, 1, 1, 1, 0, 0, 1}, // mask 3
	{1, 0, 0, 0, 1, 0, 1, 1, 0, 1, 0, 1, 1, 1, 0}, // mask 4
	{1, 0, 0, 0, 0, 0, 0, 1, 1, 0, 1, 0, 0, 0, 0}, // mask 5
	{1, 0, 0, 1, 1, 1, 1, 0, 1, 1, 1, 1, 0, 1, 1}, // mask 6
	{1, 0, 0, 1, 0, 1, 0, 0, 0, 0, 0, 0, 1, 0, 1}, // mask 7
}

func maskFunc(mask int, x, y int) bool {
	switch mask {
	case 0:
		return (x+y)%2 == 0
	case 1:
		return y%2 == 0
	case 2:
		return x%3 == 0
	case 3:
		return (x+y)%3 == 0
	case 4:
		return (y/2+x/3)%2 == 0
	case 5:
		return (x*y)%2+(x*y)%3 == 0
	case 6:
		return ((x*y)%2+(x*y)%3)%2 == 0
	case 7:
		return ((x+y)%2+(x*y)%3)%2 == 0
	}
	return false
}

func isData(v byte) bool {
	return v <= 1 // 0 or 1 are data modules
}

func qrApplyBestMask(mods [][]byte) {
	bestMask := 0
	bestScore := 1<<31 - 1
	for m := 0; m < 8; m++ {
		// Apply mask.
		for y := 0; y < qrSize; y++ {
			for x := 0; x < qrSize; x++ {
				if isData(mods[y][x]) && maskFunc(m, x, y) {
					mods[y][x] ^= 1
				}
			}
		}
		score := qrPenaltyScore(mods)
		// Undo mask.
		for y := 0; y < qrSize; y++ {
			for x := 0; x < qrSize; x++ {
				if isData(mods[y][x]) && maskFunc(m, x, y) {
					mods[y][x] ^= 1
				}
			}
		}
		if score < bestScore {
			bestScore = score
			bestMask = m
		}
	}
	// Apply best mask permanently.
	for y := 0; y < qrSize; y++ {
		for x := 0; x < qrSize; x++ {
			if isData(mods[y][x]) && maskFunc(bestMask, x, y) {
				mods[y][x] ^= 1
			}
		}
	}
	// Write format info.
	qrWriteFormatInfo(mods, bestMask)
	// Convert reserved modules: 2→0 (white), 3→1 (black).
	for y := 0; y < qrSize; y++ {
		for x := 0; x < qrSize; x++ {
			if mods[y][x] == 2 {
				mods[y][x] = 0
			} else if mods[y][x] == 3 {
				mods[y][x] = 1
			}
		}
	}
}

func qrWriteFormatInfo(mods [][]byte, mask int) {
	fb := formatBits[mask]
	// Horizontal: left of top-left finder.
	positions := [15][2]int{
		{0, 8}, {1, 8}, {2, 8}, {3, 8}, {4, 8}, {5, 8}, {7, 8}, {8, 8},
		{8, 7}, {8, 5}, {8, 4}, {8, 3}, {8, 2}, {8, 1}, {8, 0},
	}
	for i, pos := range positions {
		v := byte(2)
		if fb[i] == 1 {
			v = 3
		}
		mods[pos[1]][pos[0]] = v
	}
	// Vertical: below top-right + right of bottom-left.
	positions2 := [15][2]int{
		{8, qrSize - 1}, {8, qrSize - 2}, {8, qrSize - 3}, {8, qrSize - 4},
		{8, qrSize - 5}, {8, qrSize - 6}, {8, qrSize - 7},
		{qrSize - 8, 8}, {qrSize - 7, 8}, {qrSize - 6, 8}, {qrSize - 5, 8},
		{qrSize - 4, 8}, {qrSize - 3, 8}, {qrSize - 2, 8}, {qrSize - 1, 8},
	}
	for i, pos := range positions2 {
		v := byte(2)
		if fb[i] == 1 {
			v = 3
		}
		mods[pos[1]][pos[0]] = v
	}
}

func qrPenaltyScore(mods [][]byte) int {
	score := 0
	// Rule 1: 5+ same-color in a row/column.
	for y := 0; y < qrSize; y++ {
		run := 1
		for x := 1; x < qrSize; x++ {
			if (mods[y][x]&1) == (mods[y][x-1]&1) {
				run++
			} else {
				if run >= 5 {
					score += run - 2
				}
				run = 1
			}
		}
		if run >= 5 {
			score += run - 2
		}
	}
	for x := 0; x < qrSize; x++ {
		run := 1
		for y := 1; y < qrSize; y++ {
			if (mods[y][x]&1) == (mods[y-1][x]&1) {
				run++
			} else {
				if run >= 5 {
					score += run - 2
				}
				run = 1
			}
		}
		if run >= 5 {
			score += run - 2
		}
	}
	// Rule 2: 2x2 same-color blocks.
	for y := 0; y < qrSize-1; y++ {
		for x := 0; x < qrSize-1; x++ {
			c := mods[y][x] & 1
			if (mods[y][x+1]&1) == c && (mods[y+1][x]&1) == c && (mods[y+1][x+1]&1) == c {
				score += 3
			}
		}
	}
	// Rule 4: proportion of dark modules.
	dark := 0
	total := qrSize * qrSize
	for y := 0; y < qrSize; y++ {
		for x := 0; x < qrSize; x++ {
			if mods[y][x]&1 == 1 {
				dark++
			}
		}
	}
	pct := dark * 100 / total
	dev := pct - 50
	if dev < 0 {
		dev = -dev
	}
	score += (dev / 5) * 10
	return score
}
