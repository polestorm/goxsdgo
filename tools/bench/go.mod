module github.com/polestorm/goxsdgo/tools/bench

go 1.27.0

require (
	github.com/cB-Sigma/xsd v1.0.0
	github.com/jacoelho/xsd v0.6.15
	github.com/knroy/go-xml/v2 v2.3.1
	github.com/polestorm/goxsdgo v0.0.0
)

require (
	github.com/polestorm/goxmlgo v0.5.0 // indirect
	golang.org/x/text v0.41.0 // indirect
)

replace github.com/polestorm/goxsdgo => ../..
