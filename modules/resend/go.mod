module github.com/lunogram/platform/modules/resend

go 1.25.7

require (
	github.com/extism/go-pdk v1.1.3
	github.com/lunogram/platform v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.11.1
)

require github.com/google/uuid v1.6.0 // indirect

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/resend/resend-go/v3 v3.2.0
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/lunogram/platform => ../../
