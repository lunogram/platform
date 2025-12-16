package templates

//go:generate sh -c "command -v tailwindcss >/dev/null 2>&1 || (curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64 && chmod +x tailwindcss-linux-x64 && sudo mv tailwindcss-linux-x64 /usr/local/bin/tailwindcss)"
//go:generate tailwindcss -i ./input.css -o ./static/styles.css --minify
