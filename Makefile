.PHONY: gen-front
gen-front:
	npx tailwindcss -i ./static/styles.dev.css -o ./static/styles.css