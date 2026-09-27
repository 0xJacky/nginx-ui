package acmehint

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/miekg/dns"
)

// dnsCAALookuper queries CAA records with miekg/dns against a fixed set of
// recursive nameservers.
type dnsCAALookuper struct {
	servers []string
	client  *dns.Client
}

// newDefaultCAALookuper uses the configured recursive nameservers, or the
// nameservers from /etc/resolv.conf. It returns nil when no nameserver is
// known, which disables the CAA check.
func newDefaultCAALookuper(nameservers []string, timeout time.Duration) CAALookuper {
	servers := withDNSPort(nameservers)
	if len(servers) == 0 {
		if cfg, err := dns.ClientConfigFromFile("/etc/resolv.conf"); err == nil {
			for _, s := range cfg.Servers {
				servers = append(servers, net.JoinHostPort(s, cfg.Port))
			}
		}
	}
	if len(servers) == 0 {
		return nil
	}
	return &dnsCAALookuper{servers: servers, client: &dns.Client{Timeout: timeout}}
}

// LookupCAA implements CAALookuper.
func (l *dnsCAALookuper) LookupCAA(ctx context.Context, name string) ([]CAARecord, error) {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(name), dns.TypeCAA)
	msg.RecursionDesired = true

	var lastErr error
	for _, server := range l.servers {
		resp, _, err := l.client.ExchangeContext(ctx, msg, server)
		if err != nil {
			lastErr = err
			continue
		}
		switch resp.Rcode {
		case dns.RcodeSuccess:
			var records []CAARecord
			for _, rr := range resp.Answer {
				if caa, ok := rr.(*dns.CAA); ok {
					records = append(records, CAARecord{Flag: caa.Flag, Tag: caa.Tag, Value: caa.Value})
				}
			}
			return records, nil
		case dns.RcodeNameError:
			return nil, nil
		default:
			lastErr = fmt.Errorf("CAA lookup for %s: %s", name, dns.RcodeToString[resp.Rcode])
		}
	}
	return nil, lastErr
}
