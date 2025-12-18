package templates

//go:generate sh -c "cd ../../../../../../../../ && make $(MAKE_FLAGS) bin/tailwindcss"
//go:generate sh -c "cd ../../../../../../../../ && bin/tailwindcss -i ./services/nexus/internal/http/controllers/v1/public/templates/input.css -o ./services/nexus/internal/http/controllers/v1/public/templates/static/styles.css --minify"
