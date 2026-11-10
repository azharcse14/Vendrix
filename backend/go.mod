module vendrix/backend

// Ekmatro bahirer dependency github.com/jackc/pgx/v5 (2.1).
go 1.25.1

// v1.3: Go 1.25 line 1.25.14 e shesh (ar security patch nai); build ar CI Go 1.27.x e.
toolchain go1.27.3

require github.com/jackc/pgx/v5 v5.12.0

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.3 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/text v0.41.0 // indirect
)
