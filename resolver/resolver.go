package resolver

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type dnsrecord struct {
	CNAME  bool
	record string
}
type result struct {
	Host    string
	Results []dnsrecord
}
type toResolve struct {
	Host string
}
type Resolver struct {
	Input         string
	Resolvers     []string
	ResolversFile string
	Concurrency   int
	Hosts         []string
	ReadTimeout   time.Duration // defaults to 2 seconds
	WriteTimeout  time.Duration // defaults to 2 seconds
	delay         time.Duration // defaults 5 milliseconds (5000000 nano seconds)
}

func New() *Resolver {
	return &Resolver{ReadTimeout: time.Second * 2, WriteTimeout: time.Second * 2}
}
func (r *Resolver) SetResolversFile(ResolversFile string) error {
	if _, err := os.Stat(ResolversFile); err != nil {
		return fmt.Errorf("error, invalid resolvers file: %v", err)
	}
	r.ResolversFile = ResolversFile
	return nil
}
func (r *Resolver) SetConcurrency(i int) error {
	if i > 0 {
		r.Concurrency = i
		return nil
	}
	return errors.New("cannot set concurrent workers, number must be greater than 0.")
}
func (r *Resolver) SetInputFile(File string) error {
	_, err := os.Stat(File)
	if err != nil {
		return fmt.Errorf("Could not set input file: %s", err.Error())
	}
	r.Input = File
	return nil
}
func (r *Resolver) SetMaxPPS(Packets int) error {
	if Packets > 0 {
		r.delay = time.Duration(1000000000/Packets) * time.Nanosecond
		return nil
	}
	return errors.New("packets per second must be greater than 0")
}
func (r *Resolver) Resolve() error {
	if err := r.readResolvers(); err != nil {
		return err
	}
	var jobWg, resultWg sync.WaitGroup
	jobChan := make(chan string)
	resultChan := make(chan result)
	// concurrent workers spawn how ever many resolvers
	// there are * the num threads. should make this different
	for i := 0; i < r.Concurrency; i++ {
		jobWg.Add(1)
		go func() {
			for _, resolver := range r.Resolvers {
				c, err := net.Dial("udp", resolver)
				if err != nil {
					if strings.Contains(err.Error(), "too many open files") {
						time.Sleep(time.Second * 5)
					} else {
						fmt.Fprintf(os.Stderr, "bind(udp,%s) error: %v", resolver, err)
						continue
					}
				}
				r.doresolve(c, jobChan, resultChan)
			}
			jobWg.Done()
		}()
	}

	resultWg.Add(1)
	go func() {
		for res := range resultChan {
			fmt.Printf("%s -> ", res.Host)
			for _, result := range res.Results {
				if result.CNAME {
					fmt.Printf("%s -> ", result.record)
				} else {
					fmt.Printf("%s ", result.record)
				}
			}
			fmt.Println()
		}
		resultWg.Done()
	}()
	var b *bufio.Scanner
	switch r.Input {
	case "":
		b = bufio.NewScanner(os.Stdin)
	default:
		ifp, err := os.Open(r.Input)
		if err != nil {
			return fmt.Errorf("error opening input file: %s", err.Error())
		}
		b = bufio.NewScanner(ifp)
	}
	go func() {
		for b.Scan() {
			server := b.Text()
			if !isDomainName(server) {
				continue
			}
			jobChan <- server
		}
		close(jobChan)
	}()

	jobWg.Wait()
	close(resultChan)
	resultWg.Wait()

	return nil
}
func (r *Resolver) readResolvers() error {
	rfp, err := os.Open(r.ResolversFile)
	if err != nil {
		return fmt.Errorf("could not open resolvers file: %v", err)
	}
	defer rfp.Close()
	sc := bufio.NewScanner(rfp)
	for sc.Scan() {
		resolver := sc.Text()
		if strings.Contains(resolver, ":") {
			r.Resolvers = append(r.Resolvers, resolver)
		} else {
			r.Resolvers = append(r.Resolvers, fmt.Sprintf("%s:53", resolver))
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("error reading in resolvers: %v", err)
	}
	return nil
}
func (r *Resolver) doresolve(c net.Conn, hostChan chan string, resultChan chan result) {
	for host := range hostChan {
		in, err := r.exchange(c, msg(host))
		if err != nil {
			if strings.HasSuffix(err.Error(), "i/o timeout") {
				//host.Retries--
				continue
			} else {
				fmt.Fprintf(os.Stderr, "exchange error: %v\n", err)
				continue
			}
		}
		if in != nil && in.Rcode != dns.RcodeSuccess {
			continue
		}
		res := result{Host: host}
		// if answer is 1 then assume it's an A record.
		if len(in.Answer) == 1 {
			if re, ok := in.Answer[0].(*dns.A); ok {
				res.Results = append(res.Results, dnsrecord{CNAME: false, record: re.A.String()})
			}
		} else {
			// otherwise answer contains multiple records.
			for _, record := range in.Answer {
				switch rec := record.(type) {
				case *dns.CNAME:
					res.Results = append(res.Results, dnsrecord{CNAME: true, record: rec.Target})
				case *dns.A:
					res.Results = append(res.Results, dnsrecord{CNAME: false, record: rec.A.String()})
				}
			}
		}
		// if results aren't empty, send them back.
		if len(res.Results) > 0 {
			resultChan <- res
		}
		res.Host = ""
		res.Results = nil
		time.Sleep(r.delay)
	}
}

// creates a new dns message and returns it
func msg(host string) *dns.Msg {
	m := &dns.Msg{}
	m.Id = dns.Id()
	m.RecursionDesired = true
	m.Question = make([]dns.Question, 1)
	m.Question[0] = dns.Question{Name: dns.Fqdn(host), Qtype: dns.TypeA, Qclass: dns.ClassINET}
	return m
}

func (r *Resolver) exchange(c net.Conn, m *dns.Msg) (res *dns.Msg, err error) {
	co := new(dns.Conn)
	co.Conn = c
	co.SetReadDeadline(time.Now().Add(r.WriteTimeout))
	if err = co.WriteMsg(m); err != nil {
		if e, ok := err.(net.Error); !ok || !e.Timeout() {
			return nil, e
		}
		return nil, err
	}
	co.SetReadDeadline(time.Now().Add(r.ReadTimeout))
	res, err = co.ReadMsg()
	if e, ok := err.(net.Error); !ok || !e.Timeout() {
		return res, e
	}
	if err == nil && res.Id != m.Id {
		err = dns.ErrId
	}
	return res, err
}

// from: https://github.com/majek/goplayground/blob/73ec9678fd70a04f3afdcd1b63ce66aec4d812fc/resolve/dnsclient.go#L118
func isDomainName(s string) bool {
	// See RFC 1035, RFC 3696.
	if len(s) == 0 {
		return false
	}
	if len(s) > 255 {
		return false
	}

	last := byte('.')
	ok := false // Ok once we've seen a letter.
	partlen := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		default:
			return false
		case 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || c == '_':
			ok = true
			partlen++
		case '0' <= c && c <= '9':
			// fine
			partlen++
		case c == '-':
			// Byte before dash cannot be dot.
			if last == '.' {
				return false
			}
			partlen++
		case c == '.':
			// Byte before dot cannot be dot, dash.
			if last == '.' || last == '-' {
				return false
			}
			if partlen > 63 || partlen == 0 {
				return false
			}
			partlen = 0
		}
		last = c
	}
	if last == '-' || partlen > 63 {
		return false
	}

	return ok
}
