# certs/

This directory contains a CA certificate (`pebble.minica.pem`) and a private key
(`pebble.minica.key.pem`) that are used to issue a end-entity certificate (See
`certs/localhost`)  for the Pebble HTTPS server.

To get your **testing code** to use Pebble without HTTPS errors you should
configure your ACME client to trust the `pebble.minica.pem` CA certificate. Your
ACME client should offer a runtime option to specify a list of root CAs that you
can configure to include the `pebble.minica.pem` file.

**Do not** add this CA certificate to the system trust store or in production
code!!! The CA's private key is **public** and anyone can use it to issue
certificates that will be trusted by a system with the Pebble CA in the trust
store.

To re-create all of the Pebble certificates run:

    minica -ca-cert pebble.minica.pem \
           -ca-key pebble.minica.key.pem \
           -domains localhost,pebble \
           -ip-addresses 127.0.0.1

From the `test/certs/` directory after [installing
MiniCA](https://github.com/jsha/minica#installation)
