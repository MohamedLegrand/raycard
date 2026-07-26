module raycard

go 1.25.0

// NOTE: cette machine de développement n'a pas d'accès réseau. Les deux
// dépendances ci-dessous (fiber, zerolog) n'étaient pas dans le cache
// Go local et n'ont donc pas pu être résolues/vérifiées ici : leurs
// versions sont indicatives. Lance `go mod tidy` avec un accès réseau
// pour les corriger et compléter go.sum avant `go build ./...`.
require (
	github.com/gofiber/fiber/v2 v2.52.6
	github.com/rs/zerolog v1.33.0
)

require (
	github.com/go-playground/validator/v10 v10.30.2
	github.com/jackc/pgx/v5 v5.8.0
	github.com/joho/godotenv v1.5.1
	github.com/stretchr/testify v1.11.1
	golang.org/x/crypto v0.50.0
)

require (
	github.com/andybalholm/brotli v1.1.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gabriel-vasile/mimetype v1.4.13 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/rogpeppe/go-internal v1.15.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.51.0 // indirect
	github.com/valyala/tcplisten v1.0.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
