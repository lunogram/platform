package scalar

import (
	"embed"
	"fmt"
	"net/http"
)

//go:embed index.html
var FS embed.FS

func HandleOAPI(content []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-yaml")
		fmt.Fprint(w, string(content))
	}
}
