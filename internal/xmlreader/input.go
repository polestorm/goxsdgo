package xmlreader

import (
	"io"
	"unicode/utf8"
)

// input dekoduje bajty dokumentu do UTF-8 i normalizuje końce wierszy (XML 1.0, 2.11): każde CR i para CR LF
// stają się jednym LF. Dekodowanie przerywa na pierwszym błędnym bajcie (bad); czytnik zgłasza wtedy błąd
// w miejscu, do którego dane były poprawne.
type input struct {
	r      io.Reader
	raw    []byte
	rs, re int
	eof    bool
	ioErr  error
	bad    bool
	enc    encoding
	// afterCR: ostatni zdekodowany znak to CR zamieniony na LF; następny LF należy pominąć.
	afterCR bool
}

type encoding uint8

const (
	encUTF8 encoding = iota
	encUTF16LE
	encUTF16BE
	encLatin1
	encASCII
)

const chunkSize = 64 << 10

func newInput(r io.Reader) *input {
	return &input{r: r, raw: make([]byte, chunkSize)}
}

// done mówi, czy nie da się już dostarczyć nowych danych.
func (in *input) done() bool {
	return in.bad || in.ioErr != nil || (in.eof && in.rs == in.re)
}

// readRaw dociąga surowe bajty, zachowując niezdekodowaną resztę.
func (in *input) readRaw() {
	if in.eof || in.ioErr != nil {
		return
	}
	if in.rs > 0 {
		n := copy(in.raw, in.raw[in.rs:in.re])
		in.rs, in.re = 0, n
	}
	for tries := 0; tries < 100; tries++ {
		n, err := in.r.Read(in.raw[in.re:])
		in.re += n
		if err == io.EOF {
			in.eof = true
			return
		}
		if err != nil {
			in.ioErr = err
			return
		}
		if n > 0 {
			return
		}
	}
	in.ioErr = io.ErrNoProgress
}

// peekRaw zwraca do n pierwszych surowych bajtów bez zużywania (wykrywanie kodowania).
func (in *input) peekRaw(n int) []byte {
	for in.re-in.rs < n && !in.eof && in.ioErr == nil {
		in.readRaw()
	}
	end := min(in.rs+n, in.re)
	return in.raw[in.rs:end]
}

func (in *input) skipRaw(n int) { in.rs += n }

// decode dopisuje do *dst kolejną porcję zdekodowanego tekstu (co najmniej jeden bajt, chyba że done()).
func (in *input) decode(dst *[]byte) {
	for !in.done() {
		if in.rs == in.re || (in.enc == encUTF16LE || in.enc == encUTF16BE) && in.re-in.rs < 4 && !in.eof {
			in.readRaw()
			if in.rs == in.re {
				continue
			}
		}
		before := len(*dst)
		switch in.enc {
		case encUTF8:
			in.decodeUTF8(dst)
		case encUTF16LE, encUTF16BE:
			in.decodeUTF16(dst)
		case encLatin1, encASCII:
			in.decodeSingle(dst)
		}
		if len(*dst) > before || in.bad {
			return
		}
	}
}

func (in *input) decodeUTF8(dst *[]byte) {
	src := in.raw[in.rs:in.re]
	in.rs = in.re
	*dst = in.appendNormalized(*dst, src)
}

// appendNormalized dopisuje bajty UTF-8 z normalizacją końców wierszy. Poprawność UTF-8 sprawdza czytnik przy
// dekodowaniu znaków.
func (in *input) appendNormalized(dst, src []byte) []byte {
	for len(src) > 0 {
		if in.afterCR {
			in.afterCR = false
			if src[0] == '\n' {
				src = src[1:]
				continue
			}
		}
		i := indexCR(src)
		if i < 0 {
			return append(dst, src...)
		}
		dst = append(dst, src[:i]...)
		dst = append(dst, '\n')
		in.afterCR = true
		src = src[i+1:]
	}
	return dst
}

func indexCR(b []byte) int {
	for i, c := range b {
		if c == '\r' {
			return i
		}
	}
	return -1
}

func (in *input) appendRune(dst []byte, r rune) []byte {
	if in.afterCR {
		in.afterCR = false
		if r == '\n' {
			return dst
		}
	}
	if r == '\r' {
		in.afterCR = true
		return append(dst, '\n')
	}
	return utf8.AppendRune(dst, r)
}

func (in *input) decodeUTF16(dst *[]byte) {
	src := in.raw[in.rs:in.re]
	i := 0
	unit := func(j int) rune {
		if in.enc == encUTF16LE {
			return rune(src[j]) | rune(src[j+1])<<8
		}
		return rune(src[j])<<8 | rune(src[j+1])
	}
	for i+2 <= len(src) {
		u := unit(i)
		switch {
		case u >= 0xD800 && u < 0xDC00:
			if i+4 > len(src) {
				if in.eof {
					in.bad = true
				}
				in.rs += i
				return
			}
			u2 := unit(i + 2)
			if u2 < 0xDC00 || u2 > 0xDFFF {
				in.bad = true
				in.rs += i
				return
			}
			*dst = in.appendRune(*dst, 0x10000+(u-0xD800)<<10+(u2-0xDC00))
			i += 4
		case u >= 0xDC00 && u <= 0xDFFF:
			in.bad = true
			in.rs += i
			return
		default:
			*dst = in.appendRune(*dst, u)
			i += 2
		}
	}
	in.rs += i
	if in.eof && in.rs < in.re && in.re-in.rs < 2 {
		in.bad = true
	}
}

func (in *input) decodeSingle(dst *[]byte) {
	src := in.raw[in.rs:in.re]
	for i, b := range src {
		if b >= 0x80 && in.enc == encASCII {
			in.bad = true
			in.rs += i
			return
		}
		*dst = in.appendRune(*dst, rune(b))
	}
	in.rs = in.re
}

// switchEncoding przełącza kodowanie po deklaracji XML. Część bufora za deklaracją była już przepisana jak UTF-8
// (czyli bajt w bajt, z normalizacją końców wierszy, które w kodowaniach jednobajtowych mają te same bajty), więc
// przekodowujemy ją na miejscu.
func (in *input) switchEncoding(enc encoding, buf []byte, pos int) []byte {
	in.enc = enc
	tail := append([]byte(nil), buf[pos:]...)
	buf = buf[:pos]
	for _, b := range tail {
		if b >= 0x80 && enc == encASCII {
			in.bad = true
			// Reszta (od złego bajtu) nie trafia do bufora; czytnik zgłosi błąd, gdy do niej dojdzie.
			in.rs, in.re = 0, 0
			return buf
		}
		buf = utf8.AppendRune(buf, rune(b))
	}
	return buf
}
