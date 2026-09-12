# Upstream availability checks

Dynamic service checks use the configured DNS resolver for SRV records and target
address lookups. Resolved sockets must use `net.JoinHostPort` so IPv6 addresses
remain bracketed for the TCP availability probe. This also applies to the address
lookup fallback, which uses port 80; SRV results retain the advertised port.

`TestResolveServiceFormatsIPv6Addresses` exercises the production resolver with a
loopback DNS server. It covers IPv4 and IPv6 answers through both resolution paths
without querying public DNS.
