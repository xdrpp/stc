% stc(1)
% David Mazi&egrave;res
%

# NAME

stc - Stellar transaction compiler

# SYNOPSIS

stc [-net=_id_] [-sign] [-c] [-l] [-u] [-i | -o FILE] _input-file_ \
stc -edit [-net=ID] _file_ \
stc -post [-net=ID] _input-file_ \
stc -preauth [-net=ID] _input-file_ \
stc -txhash [-net=ID] _input-file_ \
stc -qa [-net=ID] _accountID_ \
stc -fee-stats \
stc -create [-net=ID] _accountID_ \
stc -keygen [_name_] \
stc -sec2pub [_name_] \
stc -import-key _name_ \
stc -export-key _name_ \
stc -list-keys

# DESCRIPTION

The Stellar transaction compiler, stc, is a command-line tool for
creating, viewing, editing, signing, and posting Stellar network
transactions.  It is intended for use by scripts or for creating test
transactions without the ambiguity of higher-layer wallet
abstractions.  stc is also useful in non-graphical environments, such
as a single-board computer used for cold storage.

The tool runs in one of several modes.  The default mode processes a
transaction in a single shot, optionally updating the sequence numbers
and fees, translating the transaction to/from human-readable form, or
signing it.  In edit mode, stc repeatedly invokes a text editor to
allow somewhat interactive editing of transactions.  In hash mode, stc
hashes a transactions to facilitate creation of pre-signed
transactions or lookup of transaction results.  In post mode, stc
posts a transaction to the network.  Finally, key management mode
allows one to maintain a set of signing keys, while network query mode
allows one to query the network for account and fee status.

## Default mode

The default mode parses a transaction (in either textual or
base64-encoded binary), and then outputs it.  The input comes from a
file specified on the command line, or from standard input of the
argument is "`-`".  By default, stc outputs transactions in the
human-readable _txrep_ format, specified by SEP-0011.  With the `-c`
flag, stc outputs base64-encoded binary XDR format.  Various options
modify the transaction as it is being processed, notably `-sign`,
`-key` (which implies `-sign`), and `-u`.

Txrep format is automatically derived from the XDR specification of
`TransactionEnvelope`, with just a few special-cased types.  The
format is a series of lines of the form "`Field-Name: Value Comment`".
The field name is the XDR field name, or one of two pseudo-fields.
Pointers have a boolean pseudofield called `_present` that is true
when the pointer is non-null.  Variable-length arrays have an integer
pseudofield `len` specifying the array length.  There must be no space
between a field name and the colon.  After the colon comes the value
for that field.  Anything after the value is ignored.  stc sometimes
places a comment there, such as when an account ID has been configured
to have a comment (see the FILES section below).

Two field types have specially formatted values:

* Account IDs and Signers are expressed using Stellar's "strkey"
  format, which is a base32-encoded format where public keys start
  with "G", pre-auth transaction hashes start with "T", and hash-X
  signers start with "X".  (Private keys start with "S" in strkey
  format, but never appear in transactions.)

* Asset codes are formatted as printable ASCII bytes and two-byte hex
  escapes (e.g., `\x1f`), with no surrounding quotes.  Backslash must
  be escaped with itself (e.g., `\\`).

Note that txrep is more likely to change than the base-64 XDR encoding
of transactions.  Hence, if you want to preserve transactions that you
can later read or re-use, compile them with `-c`.  XDR is also
compatible with other tools.  Notably, you can examine the contents of
an XDR transaction with `stellar-core` itself, using the command
"`stellar-core print-xdr --filetype tx --base64 FILE`", or by using
the web-based Stellar XDR viewer at
<https://www.stellar.org/laboratory/#xdr-viewer>.  You can also sign
XDR transactions with `stellar-core`, using "`stellar-core
sign-transaction --base64 --netid "Public Global Stellar Network ;
September 2015" FILE`".

## Edit mode

Edit mode is selected whenever stc is invoked with the `-edit` flag.
In this mode, whether the transaction is originally in base64 binary
or text, it is output in text format to a temporary file, and your
editor is repeatedly invoked to edit the file.  In this way, you can
change union discriminant values or array sizes, quit the editor, and
automatically re-enter the editor with any new fields appropriately
populated.

Note that for enum fields, if you add a question mark ("?") to the end
of the line, stc will populate the line with a comment containing all
possible values.  This is handy if you forget the various options to a
union discriminant such as the operation type.

Edit mode terminates when you quit the editor without modifying the
file, at which point stc writes the transaction back to the original
file.

## Post mode

Post-mode submits a transaction to the Stellar network.  This is how
you actually execute a transaction you have properly formatted and
signed.

## Hash mode

Stellar hashes transactions to a unique 32-byte value that depends on
the network identification string.  A transaction's hash, in hex
format, can be used to query horizon for the results of the
transaction after it executes.  With the option `-txhash`, stc hashes
transaction and outputs this hex value.

Stellar also allows an account to be configured to allow a
pre-authorized transaction to have a specific signing weight.  These
pre-authorized transactions use the same network-dependent hash values
as computed by `-txhash`.  However, to include such a hash as an
account signer, it must be encoded in strkey format starting with the
letter "T".  Running stc with the `-preauth` flag prints this
strkey-format hash to standard output.

Great care must be taken when creating a pre-authorized transaction,
as any mistake will cause the transaction not to run.  In particular,
make sure you have set the sequence number to one more than it will be
at the time you run the transaction, not one more than it is
currently.  (If the transaction adding the pre-authorized transaction
as a signer uses the same source account, it will consume a sequence
number.)  You should also make sure the transaction fee is high
enough.  You may wish to increase the fee above what is currently
required in case the fee has increased at the time you need to execute
the pre-authorized transaction.

Another potential source of error is that the pre-authorized
transaction hash depends on the network name, so make absolutely sure
the `-net` option is correct when using `-preauth`.

## Key management mode

stc runs in key management mode when one of the following flags is
selected:  `-keygen`, `-sec2pub`, `-import-key`, `-export-key`, and
`-list-keys`.

These options take a key name.  If the key name contains a slash, it
refers to a file in the file system.  If the key name does not contain
a slash, it refers to a file name in the stc configuration directory
(see FILES below).  This allows keys to be stored in the configuration
directory and then accessed from any directory in which stc runs.

The `-keygen` and `-sec2pub` options can be run with no key name, in
which case `-keygen` will output both the secret and public key to
standard output, and `-sec2pub` will read a key from standard input or
prompt for one to be pasted into the terminal.

Keys are generally stored encrypted, but if you supply an empty
passphrase, they will be stored in plaintext.  If you use the
`-nopass` option, stc will never prompt for a passphrase and always
assume you do not encrypt your private keys.

## Network query mode

stc runs in network query mode when the `-fee-stats`, `-qa`, or
`-create` option is provided.  `-fee-stats` reports on recent
transaction fees.  `-qa` reports on the state of a particular account.
Unfortunately, both of these requests are parsed from horizon
responses in JSON rather than XDR format, and so are reported in a
somewhat incomparable style to txrep format.  For example, balances
are shown as a fixed-point number 10^7 times the underlying int64.
`-create` creates and funds an account (which only works when the test
network is specified).

# OPTIONS

`-c`
:	Compile the output to base64 XDR binary.  Otherwise, the default
is to output in text mode.  Only available in default mode.

`-create`
:	Create and fund an account on a network with a "friendbot" that
gives away coins.  Currently the stellar test network has such a bot
available by querying the `/friendbot?addr=ACCOUNT` path on horizon.

`-edit`
:	Select edit mode.

`-export-key`
:	Print a private key in strkey format to standard output.

`-fee-stats`
:	Dump fee stats from network

`-help`
:	Print usage information.

`-i`
:	Edit in place---overwrite the input file with the stc's output.
Only available in default mode.

`-import-key`
:	Read a private key from the terminal (or standard input) and write
it (optionally encrypted) into a file (if the name has a slash) or
into the configuration directory.

`-key` _name_
:	Specifies the name of a key to sign with.  Implies the `-sign`
option.  Only available in default mode.

`-l`
:	Learn all signers associated with an account.  Queries horizon and
stores the signers under the network's configuration directory, so
that it can verify signatures from all keys associated with the
account.  Only available in default mode.

`-keygen` [_file_]
:	Creates a new public keypair.  With no argument, prints first the
secret then the public key to standard output.  When given an
argument, writes the public key to standard output and the private key
to a file, asking for a passphrase if you don't supply `-nopass`.
Note that if file contains a '/' character, the file is taken relative
to the current working directory or root directory.  If it does not,
the file is stored in stc's configuration directory.

`-list-keys`
:	List all private keys stored under the configuration directory.

`-net` _name_
:	Specify which network to use for hashing, signing, and posting
transactions, as well as for querying signers with the `-l` option.
Two pre-defined names are "main" and "test", but you can configure
other networks as discussed in the FILES section.

`-nopass`
:	Never prompt for a passphrase, so assume an empty passphrase
anytime one is required.

`-o` _file_
:	Specify a file in which to write the output.  The default is to
send the transaction to standard output unless `-i` has been
supplied.  `-i` and `-o` are mutually exclusive, and can only be used
in default mode.

`-preauth`
:	Hash a transaction to strkey for use as a pre-auth transaction
signer.  Beware that `-net` must be set correctly or the hash will be
incorrect, since the input to the hash function includes the network
ID as well as the transaction.

`-qa`
:	Query the network for the state of a particular account.

`-sec2pub`
:	Print the public key corresponding to a particular private key.

`-sign`
:	Sign the transaction.  If no `-key` option is specified, it will
prompt for the private key on the terminal (or read it from standard
input if standard input is not a terminal).

`-txhash`
:	Like `-preauth`, but outputs the hash in hex format.  Like
`-preauth`, also gives incorrect results if `-net` is not properly
specified.

`-u`
:	Query the network to update the fee and sequence number.  The fee
depends on the number of operations, so be sure to re-run this if you
change the number of transactions.  Only available in default mode.

# EXAMPLES

`stc trans`
:	Reads a transaction from a file called `trans` and prints it to
standard output in human-readable form.

`stc -edit trans`
:	Run the editor on the text format of the transaction in file
`trans` (which can be either text or base64 XDR, or not exist yet in
which case it will be created in XDR format).  Keep editing the file
until the editor quits without making any changes.

`stc -c -i -key mykey trans`
:	Reads a transaction in file `trans`, signs it using key `mykey`,
then overwrite the `trans` file with the signed transaction in base64
format.

`stc -post trans`
:	Posts a transaction in file `trans` to the network.  The
transaction must previously have been signed.

`stc -keygen`
:	Generate a new private/public key pair and print them both to
standard output, one per line (private key first).

`stc -keygen mykey`
:	Generate a new private/public key pair.  Prompt for a passphrase.
Print the public key to standard output.  Write the private key to
`$HOME/.config/stc/keys/mykey` encrypted with the passphrase.

# ENVIRONMENT

EDITOR
:	Name of editor to invoke with the `-edit` argument (default: `vi`)

STCDIR
:	Directory containing all the configuration files (default:
`$XDG_CONFIG_HOME/stc` or `$HOME/.config/stc`)

STCNET
:	Name of network to use by default if not overridden by `-net`
argument (default: `default`)

# FILES

All configuration files reside in a configuration directory:
`$STCDIR` if that environment variable exists, `$XDG_CONFIG_HOME/stc`
if that environment variable exists, and otherwise
`$HOME/.config/stc`.  Within the configuration directory are two
subdirectories: `keys` and `networks`.

Each file in `keys` contains a signing key, which is either a single
line of text representing a Stellar signing key in strkey format
(starting with the letter "S"), or such a line of text symmetrically
encrypted and ASCII armored by gpg.  These are the key names supplied
to options such as `-key` and `-export-key`.

Within the `networks` directory are a bunch of subdirectories whose
names correspond to the _id_ argument to the `-net` option.  Within
each subdirectory of `networks` there are four files:

* `network_id` corresponds to the Stellar network ID that permutes
  signatures and pre-signed-transaction hashes (which prevents
  signatures from being valid on more than one instantiation of the
  Stellar network).  stc by default populates these files correctly
  for the main public Stellar network and automatically fetches and
  stores the network ID of test network the first time it is used.
  You probably shouldn't edit these files, but may wish to create new
  ones if you instantiate your own networks using the Stellar code
  base and don't want stc to fetch the network automatically, or if
  you relaunch a network with a different network ID, in which case
  you need to delete the old `network_id` file.

* `native_asset` shows how to render the native asset.  This defaults
  to `XLM` for the stellar main network, and `TestXLM` for the stellar
  test network.  For other networks, or if the file is just blank, it
  defaults to the string `NATIVE`.  Note that this only controls how
  the asset is rendered not parsed.  When parsing, any string not
  ending ":IssuerAccountID" is considered the native asset.

* `horizon` corresponds to the base URL of the horizon instance to use
  for this network.  You may wish to change this URL to use your own
  local validator if you are running one, or else that of an exchange
  that you trust.  Note that the URL _must_ end with a `/` (slash)
  character.

* `accounts` assigns comments to accounts, so that you don't have to
  remember account names when proofreading transactions.  The file is
  not created by default.  The format is simply a bunch of lines each
  of the form `AccountID comment`.

* `signers` remembers public signing keys and optionally assigns
  comments to them, so that stc can check the signatures in
  transactions it is processing.  This file can be populated by
  default by running the `-l` flag on a transaction (which queries
  horizon for additional signers beyond the master key).  You can also
  edit this file by hand to add comments to individual signers, which
  is particularly useful in the case of a multi-sig wallet where you
  want to see who has signed a transaction already.

# SEE ALSO

stellar-core(1), gpg(1)

The Stellar web site:  <https://www.stellar.org/>

Stellar's web-based XDR viewer:\
<https://www.stellar.org/laboratory/#xdr-viewer>

SEP-0011, the specification for txrep format:\
<https://github.com/stellar/stellar-protocol/blob/master/ecosystem/sep-0011.md>

RFC4506, the specification for XDR:\
<https://tools.ietf.org/html/rfc4506>

The XDR definition of a `TransactionEnvelope`:\
<https://github.com/stellar/stellar-core/blob/master/src/xdr/Stellar-transaction.x>

# BUGS

stc accepts and generates any `TransactionEnvelope` that is valid
according to the XDR specification.  However, a `TransactionEnvelope`
that is syntactically valid XDR may not be a valid Stellar
transaction.  stellar-core imposes additional restrictions on
transactions, such as prohibiting non-ASCII characters in certain
string fields.  This fact is important to keep in mind when using stc
to examine pre-signed transactions:  what looks like a valid, signed
transaction may not actually be valid.

stc uses a potentially imperfect heuristic to decide whether a file
contains a base64-encoded binary transaction or a textual one.

stc can only encrypt secret keys with symmetric encryption.  However,
the `-sign` option will read a key from standard input, so you can
always run `gpg -d keyfile.pgp | stc -sign -i txfile` to sign the
transaction in `txfile` with a public-key-encrypted signature key in
`keyfile.pgp`.

The options that interact with Horizon and parse JSON (such as `-qa`)
report things in a different style from the options that manipulate
XDR.

Various forms of malformed textual input will surely cause stc to
panic, though the binary parser should be pretty robust.
