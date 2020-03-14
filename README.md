# reckdns
A recursive (kinda reckless) dns resolver. **This is still under development**

## installation:
via `go get`

```bash
▻ go get github.com/lc/reckdns
```

from repo:
```
▻ git clone https://github.com/lc/reckdns && cd reckdns
▻ go build -o $GOPATH/bin/reckdns main.go
```

## usage:
```bash
▻ printf 'www.yahoo.com\nwww.google.com\nwww.amazon.com'  | reckdns -r resolvers.txt 
▻ reckdns -r resolvers.txt -i hosts.txt
```
