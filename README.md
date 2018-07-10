
# Building `stc`

To compile the program, you need `stringer`, `goyacc` and the Go
`crypto/e25519` library installed.  If you don't already have these
installed, run the following command:

    go get -u golang.org/x/crypto/ed25519 golang.org/x/tools/cmd/goyacc golang.org/x/tools/cmd/stringer

That will place the tools under `$GOPATH` or `$HOME/go` by default.
With these dependencies in place, just run:

    make

to build the tool.
