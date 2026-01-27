package oapi

import (
	_ "embed"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/lunogram/platform/services/nexus/internal/http/json"
	"github.com/lunogram/platform/services/nexus/internal/http/problem"
)

//go:generate oapi-codegen -o ./resources_gen.go -generate types,client,chi-server -package oapi ./resources.yml

//go:embed resources.yml
var oapi []byte

func Spec() (*openapi3.T, error) {
	return openapi3.NewLoader().LoadFromData(oapi)
}

// WriteProblem writes a JSON v1 problem message to the given response writer.
// The problem status, description, and other properties are unwrapped from the
// given error.
func WriteProblem(w http.ResponseWriter, err error) {
	title, description := problem.GetDescription(err)
	status := problem.GetStatus(err)
	result := Problem{
		Title:  title,
		Detail: description,
	}

	if result.Title == "" {
		result.Title = strings.ToLower(http.StatusText(status))
	}

	json.Write(w, status, result)
}

type PaginationLimit int

func (limit *PaginationLimit) ToInt() int {
	if limit == nil {
		return 20
	}

	result := int(*limit)

	if result > 100 {
		result = 100
	}

	if result < 0 {
		result = 20
	}

	return result
}

type PaginationOffset int

func (offset *PaginationOffset) ToInt() int {
	if offset == nil {
		return 0
	}

	result := int(*offset)

	if result < 0 {
		result = 0
	}

	return result
}

type PaginationSearch string

func (search *PaginationSearch) ToString() string {
	if search == nil {
		return ""
	}

	return string(*search)
}
