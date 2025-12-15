// Package main implements a Composition Function.
package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"

	"github.com/crossplane/function-sdk-go"
)

// CLI of this Function.
type CLI struct {
	Debug bool `short:"d" help:"Emit debug logs in addition to info logs."`

	Network            string `help:"Network on which to listen for gRPC connections." default:"tcp"`
	Address      	   string `help:"Address at which to listen for gRPC connections." default:":9443"`
	TLSCertsDir  	   string `help:"Directory containing server certs (tls.key, tls.crt) and the CA used to verify client certificates (ca.crt)" env:"TLS_SERVER_CERTS_DIR"`
	Dependencies 	   string `help:"File containing dependencies to add to all functions."`
	Insecure     	   bool   `help:"Run without mTLS credentials. If you supply this flag --tls-server-certs-dir will be ignored."`
	MaxRecvMessageSize int    `help:"Maximum size of received messages in MB." default:"4"`
}

// Run this Function.
func (c *CLI) Run() error {
	dependencies := ""
	if c.Dependencies != "" {
		if bytes, err := os.ReadFile(c.Dependencies); err != nil {
			return fmt.Errorf("reading %q: %w", c.Dependencies, err)
		} else {
			dependencies = string(bytes)
		}
	}
	log, err := function.NewLogger(c.Debug)
	if err != nil {
		return err
	}
	return function.Serve(&Function{dependencies: dependencies, log: log},
		function.Listen(c.Network, c.Address),
		function.MTLSCertificates(c.TLSCertsDir),
		function.Insecure(c.Insecure),
		function.MaxRecvMessageSize(c.MaxRecvMessageSize*1024*1024))
}

func main() {
	ctx := kong.Parse(&CLI{}, kong.Description("A Crossplane Composition Function using KCL."))
	ctx.FatalIfErrorf(ctx.Run())
}
