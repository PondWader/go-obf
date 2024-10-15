# go-obf

Obfuscate go projects by renaming identifiers in the main package.

## Installation
```
go install github.com/pondwader/go-obf@latest
```

## Usage
```
go-obf -o ./out.exe .
```

`-go-flags` can be passed to specify flags to pass to `go build`. For example:
```
go-obf -o ./out.exe -go-flags "-v" .
```
This works by overriding the `GOFLAGS` environment variable.

## Directives
Placing `//obf:preserve-fields` above a struct declaration will prevent the field names from being obfuscated which can be neccesary if use with reflection.
```go
//obf:preserve-fields
type Example struct {
    A string
    B int
}
```
Placing `//obf:protect` above a string variable will protect the string value from being dumped from the resulting binary.
```go
//obf:protect
var ProtectedString = "hi"
```
