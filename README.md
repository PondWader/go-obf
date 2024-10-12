The plan
- all identifiers get replaced with the same thing through the project
- //obf:preserve-fields will preserve the field names of a struct
- //obf:protect before a constant string will make  a generator function for it 

new plan new plan new plan
all imports get crawled, any public values found in structs or function names or whatever, get added to the exclude idents
Perhaps can use `golang.org/x/tools/go/packages`
as seen here https://github.com/golang/example/blob/master/gotypes/doc/main.go

## CACHE IMPORTS