package xmlreader

import (
	"encoding/xml"
	"io"
)

// stdDecoder: encoding/xml jako punkt odniesienia w benchmarku.
func stdDecoder(r io.Reader) *xml.Decoder { return xml.NewDecoder(r) }
