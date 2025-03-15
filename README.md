# `nanodb` — the stupidest db I could imagine

~100 lines of code, works amazing — 4 million requests / second.

```
goos: darwin
goarch: arm64
pkg: github.com/kittenbark/nanodb
cpu: Apple M2
BenchmarkDB_Sequential
BenchmarkDB_Sequential-8        4039136           287.0 ns/op
```


## Example (in-memory)

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

## Example (with file caching)

```go
package main

import "github.com/kittenbark/nanodb"

func main() {
    db, _ := nanodb.From[string]("cache.json")

    _ = db.Add("key", "value")
    println(db.Get("key"))
    _ = db.Del("key")
    if value, ok, _ := db.TryGet("key"); ok {
        println("no way, how you get this? " + value)
    }
}
```

## But why is it json?

Well, it's not.

```go
package main

import (
    "github.com/kittenbark/nanodb"
    "gopkg.in/yaml.v3"
)

func main() {
    db, _ := nanodb.Fromf[string]("cache.yaml", yaml.NewEncoder, yaml.NewDecoder)

    _ = db.Add("key", "value")
    println(db.Get("key"))
    _ = db.Del("key")
    if value, ok, _ := db.TryGet("key"); ok {
        println("no way, how you get this? " + value)
    }
}
```
