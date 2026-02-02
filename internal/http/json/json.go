package json

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/lunogram/platform/internal/http/problem"
)

const ContentType = "Content-Type"
const ApplicationJSON = "application/json"

var Marshal = json.Marshal
var Unmarshal = json.Unmarshal

type RawMessage = json.RawMessage

// Decode decodes the incoming JSON request body into the given value.
func Decode(r io.ReadCloser, v any) error {
	decoder := json.NewDecoder(r)
	defer r.Close()

	err := decoder.Decode(v)
	if err != nil {
		err = problem.WithDescription(err, "invalid request body", err.Error())
		err = problem.WithStatus(err, http.StatusBadRequest)
		return err
	}

	return nil
}

func Write(w http.ResponseWriter, status int, v any) {
	bb, err := json.Marshal(v)
	if err != nil {
		// TODO: it is probably best if we would handle the error instead
		panic(err)
	}

	w.Header().Set(ContentType, ApplicationJSON)

	w.WriteHeader(status)
	w.Write(bb) //nolint:errcheck
}
