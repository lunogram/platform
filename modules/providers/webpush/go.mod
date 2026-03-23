module github.com/lunogram/platform/modules/providers/webpush

go 1.25.1

require (
	github.com/SherClockHolmes/webpush-go v1.4.0
	github.com/extism/go-pdk v1.1.3
	github.com/lunogram/platform v0.0.0-00010101000000-000000000000
)

require (
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	golang.org/x/crypto v0.47.0 // indirect
)

replace github.com/lunogram/platform => ../../../
