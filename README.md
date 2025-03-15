# `nanodb` — the stupidest db I could imagine

Less than 100 lines of code, works amazing — 4 million requests / second.

```
goos: darwin
goarch: arm64
pkg: github.com/kittenbark/nanodb
cpu: Apple M2
BenchmarkDB_Sequential
BenchmarkDB_Sequential-8   	 4039136	       287.0 ns/op
```


## Example

```go
package main

import "github.com/kittenbark/nanodb"

func main() {
	db := nanodb.New[string]()

	db.Add("key", "value")
	println(db.Get("key"))
	db.Del("key")
	if value, ok := db.TryGet("key"); ok {
		println("no way, how you get this? " + value)
	}
}

```