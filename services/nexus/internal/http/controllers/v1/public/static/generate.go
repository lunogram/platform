package static

//go:generate sh -c "cd $(git rev-parse --show-toplevel) && pnpm exec tailwindcss -i ./services/nexus/internal/http/controllers/v1/public/static/input.css -o ./services/nexus/internal/http/controllers/v1/public/static/styles.css --config ./services/nexus/internal/http/controllers/v1/public/static/tailwind.config.js --minify"
