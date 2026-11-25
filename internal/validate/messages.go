package validate

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/polestorm/goxsdgo/internal/datatypes"
)

// valueMessage składa komunikat błędu wartości typu prostego z pól błędu pakietu datatypes: czytelne zdanie
// zamiast kodu i surowych szczegółów (ADR 0009). path i name wskazują element albo atrybut.
func valueMessage(de *datatypes.Error, path, name string) string {
	subject := "element " + name
	if strings.HasSuffix(path, "/@"+name) {
		subject = "attribute " + name
	}
	return subject + ": " + valueSentence(de)
}

func valueSentence(de *datatypes.Error) string {
	v := strconv.Quote(abbreviateValue(de.Value))
	var a, b int
	switch de.Code {
	case datatypes.CodeDatatypeAtomic:
		if de.Detail != "" {
			return "value " + v + " is " + de.Detail
		}
		return "value " + v + " is not valid for its type"
	case datatypes.CodeDatatypeList:
		var inner *datatypes.Error
		if errors.As(de.Cause, &inner) && inner != de {
			return de.Detail + " of the list " + v + ": " + valueSentence(inner)
		}
		return "value " + v + " is not a valid list"
	case datatypes.CodeDatatypeUnion:
		return "value " + v + " matches no member type of the union"
	case datatypes.CodePattern:
		return "value " + v + " does not match the pattern " + de.Detail
	case datatypes.CodeEnumeration:
		return "value " + v + " is not one of the allowed values: " + abbreviateList(strings.TrimPrefix(de.Detail, "allowed: "))
	case datatypes.CodeLength:
		if n, _ := fmt.Sscanf(de.Detail, "length %d, required %d", &a, &b); n == 2 {
			return fmt.Sprintf("value %s has length %d; the required length is %d", v, a, b)
		}
	case datatypes.CodeMinLength:
		if n, _ := fmt.Sscanf(de.Detail, "length %d, minimum %d", &a, &b); n == 2 {
			return fmt.Sprintf("value %s has length %d, less than the minimum length %d", v, a, b)
		}
	case datatypes.CodeMaxLength:
		if n, _ := fmt.Sscanf(de.Detail, "length %d, maximum %d", &a, &b); n == 2 {
			return fmt.Sprintf("value %s has length %d, more than the maximum length %d", v, a, b)
		}
	case datatypes.CodeTotalDigits:
		if n, _ := fmt.Sscanf(de.Detail, "%d digits, maximum %d", &a, &b); n == 2 {
			return fmt.Sprintf("value %s has %d digits, more than the allowed %d", v, a, b)
		}
	case datatypes.CodeFractionDigits:
		if n, _ := fmt.Sscanf(de.Detail, "%d fraction digits, maximum %d", &a, &b); n == 2 {
			return fmt.Sprintf("value %s has %d fraction digits, more than the allowed %d", v, a, b)
		}
	case datatypes.CodeMinInclusive:
		return "value " + v + " is less than the minimum " + strings.TrimPrefix(de.Detail, "minInclusive ")
	case datatypes.CodeMaxInclusive:
		return "value " + v + " is greater than the maximum " + strings.TrimPrefix(de.Detail, "maxInclusive ")
	case datatypes.CodeMinExclusive:
		return "value " + v + " must be greater than " + strings.TrimPrefix(de.Detail, "minExclusive ")
	case datatypes.CodeMaxExclusive:
		return "value " + v + " must be less than " + strings.TrimPrefix(de.Detail, "maxExclusive ")
	}
	if de.Detail != "" {
		return "value " + v + " violates " + de.Code + ": " + de.Detail
	}
	return "value " + v + " violates " + de.Code
}

// abbreviateValue skraca długą wartość w komunikacie; pełna wartość jest w polu Value.
func abbreviateValue(s string) string {
	const max = 200
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "…"
}

// abbreviateList skraca długą listę dozwolonych wartości w komunikacie; pełna lista jest w schemacie.
func abbreviateList(s string) string {
	parts := strings.Split(s, ", ")
	if len(parts) <= 12 {
		return s
	}
	return strings.Join(parts[:12], ", ") + fmt.Sprintf(", … (%d values)", len(parts))
}
