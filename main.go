package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/lc/reckdns/resolver"
)

func main() {
	workers := flag.Int("t", 5, "number of concurrent workers")
	input := flag.String("i", "", "File to read domains from.")
	resolvers := flag.String("r", "", "path to file containing resolvers (ip:port)")
	pps := flag.Int("pps", 200, "DNS packets per second")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, strings.Join([]string{
			"reckdns - recursive (kinda reckless) dns resolver",
			"",
			"Usage: reckdns [options ...]",
			"",
		}, "\n"))
		flag.PrintDefaults()
	}
	flag.Parse()
	if *resolvers == "" {
		flag.Usage()
		os.Exit(1)
	}
	r := resolver.New()
	if err := r.SetConcurrency(*workers); err != nil {
		log.Fatal(err)
	}
	if *input != "" {
		err := r.SetInputFile(*input)
		if err != nil {
			log.Fatal(err)
		}
	}
	if err := r.SetResolversFile(*resolvers); err != nil {
		log.Fatal(err)
	}
	if err := r.SetMaxPPS(*pps); err != nil {
		log.Fatal(err)
	}
	err := r.Resolve()
	if err != nil {
		log.Fatal(err)
	}
}
