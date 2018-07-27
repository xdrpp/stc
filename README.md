# Stellar transaction compiler

This is a command-line tool for creating, viewing, editing, and
signing transactions for the [Stellar](https://www.stellar.org/)
blockchain.

# Building `stc`

To compile this program, you need `stringer`, `goyacc` and the Go
extra `crypto` library installed.  If you don't already have these
installed, run the following command:

    make build-depend

That will install these build dependencies under `$GOPATH` or
`$HOME/go` by default.  Once these dependencies are place, just run:

    make

to build the tool.

To install the tool, you will also need [pandoc](https://pandoc.org/)
to format the man page.

# Using `stc`

See the [man page](stc.1.md)

# Disclaimer

There is no warranty for the program, to the extent permitted by
applicable law.  Except when otherwise stated in writing the copyright
holders and/or other parties provide the program "as is" without
warranty of any kind, either expressed or implied, including, but not
limited to, the implied warranties of merchantability and fitness for
a particular purpose.  The entire risk as to the quality and
performance of the program is with you.  Should the program prove
defective, you assume the cost of all necessary servicing, repair or
correction.

In no event unless required by applicable law or agreed to in writing
will any copyright holder, or any other party who modifies and/or
conveys the program as permitted above, be liable to you for damages,
including any general, special, incidental or consequential damages
arising out of the use or inability to use the program (including but
not limited to loss of data or data being rendered inaccurate or
losses sustained by you or third parties or a failure of the program
to operate with any other programs), even if such holder or other
party has been advised of the possibility of such damages.
