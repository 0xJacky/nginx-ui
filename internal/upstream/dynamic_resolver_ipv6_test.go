package upstream

import (
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

// Exercise the production resolver against loopback DNS, including both SRV
// resolution and its address-lookup fallback. No public DNS queries are needed.
func TestResolveServiceFormatsIPv6Addresses(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		name := "srv"
		if fallback {
			name = "address_fallback"
		}
		t.Run(name, func(t *testing.T) {
			conn, err := net.ListenPacket("udp", "127.0.0.1:0")
			require.NoError(t, err)
			server := &dns.Server{PacketConn: conn, Handler: dns.HandlerFunc(func(w dns.ResponseWriter, request *dns.Msg) {
				response := new(dns.Msg)
				response.SetReply(request)
				for _, question := range request.Question {
					header := dns.RR_Header{Name: question.Name, Class: dns.ClassINET, Ttl: 60}
					switch question.Qtype {
					case dns.TypeSRV:
						if !fallback {
							header.Rrtype = dns.TypeSRV
							response.Answer = append(response.Answer, &dns.SRV{Hdr: header, Port: 8080, Target: "backend.example.test."})
						}
					case dns.TypeA:
						header.Rrtype = dns.TypeA
						response.Answer = append(response.Answer, &dns.A{Hdr: header, A: net.ParseIP("192.0.2.10")})
					case dns.TypeAAAA:
						header.Rrtype = dns.TypeAAAA
						response.Answer = append(response.Answer, &dns.AAAA{Hdr: header, AAAA: net.ParseIP("2001:db8::10")})
					}
				}
				_ = w.WriteMsg(response)
			})}
			started := make(chan struct{})
			server.NotifyStartedFunc = func() { close(started) }
			done := make(chan error, 1)
			go func() { done <- server.ActivateAndServe() }()
			select {
			case <-started:
			case err := <-done:
				_ = conn.Close()
				t.Fatalf("DNS server exited before startup: %v", err)
			case <-time.After(5 * time.Second):
				_ = conn.Close()
				t.Fatal("DNS server did not start")
			}
			t.Cleanup(func() {
				require.NoError(t, server.Shutdown())
				require.NoError(t, <-done)
			})

			addresses, err := NewDynamicResolver(conn.LocalAddr().String()).ResolveService("example.test service=http resolve")
			require.NoError(t, err)
			port := "8080"
			if fallback {
				port = "80"
			}
			require.ElementsMatch(t, []string{"192.0.2.10:" + port, "[2001:db8::10]:" + port}, addresses)
			for _, address := range addresses {
				_, actualPort, err := net.SplitHostPort(address)
				require.NoError(t, err)
				require.Equal(t, port, actualPort)
			}
		})
	}
}
