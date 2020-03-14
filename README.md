# reckdns
A recursive (kinda reckless) dns resolver. **This is VERYYY much still under development**

## installation:
via `go get`

```
▻ go get github.com/lc/reckdns
```

from repo:
```
▻ git clone https://github.com/lc/reckdns && cd reckdns
▻ go build -o $GOPATH/bin/reckdns main.go
```

## usage:
```
▻ printf 'www.yahoo.com\nwww.google.com\nwww.amazon.com'  | reckdns -r resolvers.txt 
▻ reckdns -r resolvers.txt -i hosts.txt
```

**Warning**:
For each concurrent thread, there will be however many workers as you have resolvers in your resolvers.txt file.

From: https://github.com/lc/reckdns/blob/af03707e918a92d215d7a3ef9e3d12895ed51140/resolver/resolver.go#L79

```go
for i := 0; i < r.Concurrency; i++ {
	...
		for _, resolver := range r.Resolvers {
			c, err := net.Dial("udp", resolver)
		...
			r.doresolve(c, jobChan, resultChan)
		}
	...
}

```
