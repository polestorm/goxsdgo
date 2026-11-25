package xmlreader

// Klasy znaków z XML 1.0 Fifth Edition, rozdział 2.2 (Char, [2]) i 2.3 (S [3], NameStartChar [4], NameChar [4a]).

// isChar odpowiada produkcji [2] Char.
func isChar(r rune) bool {
	switch {
	case r < 0x20:
		return r == 0x9 || r == 0xA || r == 0xD
	case r <= 0xD7FF:
		return true
	case r < 0xE000:
		return false
	case r <= 0xFFFD:
		return true
	case r < 0x10000:
		return false
	}
	return r <= 0x10FFFF
}

// isSpace odpowiada produkcji [3] S (jeden znak).
func isSpace(b byte) bool {
	return b == 0x20 || b == 0x9 || b == 0xA || b == 0xD
}

// asciiName: bit 1 to NameStartChar, bit 2 to NameChar, dla znaków ASCII (szybka ścieżka).
var asciiName = func() (t [128]uint8) {
	for c := 0; c < 128; c++ {
		start := c == ':' || c == '_' || ('A' <= c && c <= 'Z') || ('a' <= c && c <= 'z')
		if start {
			t[c] |= 1 | 2
		}
		if c == '-' || c == '.' || ('0' <= c && c <= '9') {
			t[c] |= 2
		}
	}
	return t
}()

// isNameStart odpowiada produkcji [4] NameStartChar.
func isNameStart(r rune) bool {
	if r < 0x80 {
		return asciiName[r]&1 != 0
	}
	switch {
	case 0xC0 <= r && r <= 0xD6, 0xD8 <= r && r <= 0xF6, 0xF8 <= r && r <= 0x2FF,
		0x370 <= r && r <= 0x37D, 0x37F <= r && r <= 0x1FFF, 0x200C <= r && r <= 0x200D,
		0x2070 <= r && r <= 0x218F, 0x2C00 <= r && r <= 0x2FEF, 0x3001 <= r && r <= 0xD7FF,
		0xF900 <= r && r <= 0xFDCF, 0xFDF0 <= r && r <= 0xFFFD, 0x10000 <= r && r <= 0xEFFFF:
		return true
	}
	return false
}

// isNameChar odpowiada produkcji [4a] NameChar.
func isNameChar(r rune) bool {
	if r < 0x80 {
		return asciiName[r]&2 != 0
	}
	return isNameStart(r) || r == 0xB7 || (0x300 <= r && r <= 0x36F) || (0x203F <= r && r <= 0x2040)
}

// isPubidChar odpowiada produkcji [13] PubidChar.
func isPubidChar(b byte) bool {
	switch {
	case b == 0x20 || b == 0xD || b == 0xA:
		return true
	case 'a' <= b && b <= 'z', 'A' <= b && b <= 'Z', '0' <= b && b <= '9':
		return true
	}
	switch b {
	case '-', '\'', '(', ')', '+', ',', '.', '/', ':', '=', '?', ';', '!', '*', '#', '@', '$', '_', '%':
		return true
	}
	return false
}

// textByte mówi, czy bajt ASCII może wystąpić w CharData bez dodatkowego sprawdzania: znak dozwolony ([2])
// i różny od '<', '&', ']' (']' sprawdzamy osobno, bo "]]>" jest zabronione w CharData, [14]).
var textByte = func() (t [256]bool) {
	for b := 0x20; b < 0x80; b++ {
		t[b] = true
	}
	t['\t'], t['\n'] = true, true
	t['<'], t['&'], t[']'] = false, false, false
	return t
}()

// isNCName sprawdza, czy s jest nazwą bez dwukropka (Namespaces in XML 1.0, [4] NCName).
func isNCName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if r == ':' || (i == 0 && !isNameStart(r)) || !isNameChar(r) {
			return false
		}
	}
	return true
}
