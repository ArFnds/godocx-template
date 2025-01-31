module github.com/ArFnds/godocx-template

go 1.23.5

replace github.com/ArFnds/godocx-template/internal => ./internal

require (
	github.com/ArFnds/godocx-template/internal v0.0.0-00010101000000-000000000000
	github.com/gomarkdown/markdown v0.0.0-20241205020045-f7e15b2f3e62
)

require golang.org/x/text v0.21.0 // indirect
