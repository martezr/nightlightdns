// Package example is a CoreDNS plugin that prints "example" to stdout on every packet received.
//
// It serves as an example CoreDNS plugin with numerous code comments.
package nightlightdns

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"strings"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

type DNSRecords struct {
	Records []DNSRecord `json:"records"`
}
type DNSRecord struct {
	Name      string `json:"name"`
	Ipaddress string `json:"ipaddress"`
}

// Define log to be a logger with the plugin name in it. This way we can just use log.Info and
// friends to log.
var log = clog.NewWithPlugin("nightlightdns")

// Example is an example plugin to show how to write a plugin.
type Nightlightdns struct {
	Next plugin.Handler
}

// ServeDNS implements the plugin.Handler interface. This method gets called when example is used
// in a Server.
func (n Nightlightdns) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	// This function could be simpler. I.e. just fmt.Println("example") here, but we want to show
	// a slightly more complex example as to make this more interesting.
	// Here we wrap the dns.ResponseWriter in a new ResponseWriter and call the next plugin, when the
	// answer comes back, it will print "example".

	var (
		err error
	)

	// Debug log that we've have seen the query. This will only be shown when the debug plugin is loaded.
	log.Debug("Received response")
	state := request.Request{W: w, Req: r}
	qname := state.Name()
	log.Info(qname)
	answers := []dns.RR{}

	// check record type here and bail out if not A or AAAA
	if state.QType() != dns.TypeA && state.QType() != dns.TypeAAAA {
		// always fallthrough if configured
		return plugin.NextOrFailure(n.Name(), n.Next, ctx, w, r)

		// otherwise return SERVFAIL here without fallthrough
		return dnserror(dns.RcodeServerFailure, state, err)
	}

	file, _ := ioutil.ReadFile("dns.json")

	data := DNSRecords{}

	_ = json.Unmarshal([]byte(file), &data)

	outip := ""
	for _, record := range data.Records {
		log.Info(record.Ipaddress)
		baseName := strings.Split(qname, ".")
		if record.Name == baseName[0] {
			log.Info(fmt.Sprintf("Found matching record: %s - %s", baseName, record.Ipaddress))
			outip = record.Ipaddress
		}
	}

	answers = append(answers, &dns.A{
		Hdr: dns.RR_Header{
			Name:   qname,
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    30,
		},
		A: net.ParseIP(outip),
	})
	log.Info(answers)

	// Export metric with the server label set to the current server handling the request.
	requestCount.WithLabelValues(metrics.WithServer(ctx)).Inc()

	// create DNS response
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true
	m.Answer = answers

	// send response back to client
	_ = w.WriteMsg(m)

	// signal response sent back to client
	return dns.RcodeSuccess, nil
}

// Name implements the Handler interface.
func (n Nightlightdns) Name() string { return "nightlightdns" }

// ResponsePrinter wrap a dns.ResponseWriter and will write example to standard output when WriteMsg is called.
type ResponsePrinter struct {
	dns.ResponseWriter
}

// NewResponsePrinter returns ResponseWriter.
func NewResponsePrinter(w dns.ResponseWriter) *ResponsePrinter {
	return &ResponsePrinter{ResponseWriter: w}
}

// WriteMsg calls the underlying ResponseWriter's WriteMsg method and prints "example" to standard output.
func (r *ResponsePrinter) WriteMsg(res *dns.Msg) error {
	log.Info("nightlightdns")
	fmt.Println("nightlightdns")
	return r.ResponseWriter.WriteMsg(res)
}

func dnserror(rcode int, state request.Request, err error) (int, error) {
	m := new(dns.Msg)
	m.SetRcode(state.Req, rcode)
	m.Authoritative = true

	// send response
	_ = state.W.WriteMsg(m)

	// return success as the rcode to signal we have written to the client.
	return dns.RcodeSuccess, err
}
