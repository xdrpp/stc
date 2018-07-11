
# Building `stc`

To compile the program, you need `stringer`, `goyacc` and the Go extra
`crypto` library installed.  If you don't already have these
installed, run the following command:

    make build-depend

That will install these build dependencies under `$GOPATH` or
`$HOME/go` by default.  Once these dependencies are place, just run:

    make

to build the tool.
