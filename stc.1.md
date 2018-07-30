% stc(1)
% David Mazieres
%

# NAME

stc - Stellar transaction compiler

# SYNOPSIS

stc [-net=_id_] [-sign] [-c] [-l] [-u] [-i | -o FILE] _input-file_ \
stc -edit [-net=ID] _file_ \
stc -post [-net=ID] _input-file_ \
stc -preauth [-net=ID] _input-file_ \
stc -keygen [_name_] \
stc -sec2pub [_name_] \
stc -import-key _name_ \
stc -export-key _name_ \
stc -list-keys

# DESCRIPTION

The Stellar transaction compiler is a command-line tool for creating,
viewing, editing, signing, and posting Stellar network transactions.
It is intended to be integrated into shell-scripts, or to create test
transactions without the ambiguity of higher-layer wallet
abstractions.  It can also be useful in non-graphical environments,
such as a single-board computer implementing cold storage.

The tool runs in one of several modes.  The default mode processes a
transaction in a single shot, optionally updating the sequence numbers
and fees, translating the transaction to/from human-readable form, or
signing it.  In edit mode, stc repeatedly invokes a text editor to
allow somewhat interactive editing of transactions.  In preauth mode,
stc hashes a transactions to facilitate creation of pre-signed
transactions.  In post mode, stc posts a transaction to the network.
Finally, key management mode allows one to maintain a set of signing
keys.

## Default mode

The default mode parses a transaction (in either textual or
base64-encoded binary), and then outputs it.  The input comes from a
file specified, or from standard input of the argument is "`-`".  By
default, a transaction is output in human-readable text form.  With
the `-c` flag, it will be output in base64-encoded binary XDR format.
Various options modify the transaction as it is being processed,
notably `-sign`, `-key` (which implies `-sign`), and `-u`.

The human-readable text form of the transaction is automatically
derived from the XDR, with just a few special-cased types.  The format
of is a series of lines of the form "`Field-Name: Value Comment`".
The field name is the XDR field name but with each component
capitalized.  There must be no space between the field name and the
colon.  After the colon comes the value for that field.  Anything
after the value is ignored.  stc sometimes places a comment there,
such as when an account ID has been configured to have a comment (see
the FILES section below).

The fields with specially formatted values are as follows:

* Account IDs and Signers are expressed using Stellar's "strkey"
  format, which is a base32-encoded format where public keys start
  with "G", pre-auth transaction hashes start with "T", and hash-X
  signers start with "X".  (Private keys start with "S" in strkey
  format, but never appear in transactions.)

* Asset codes are formatted as printable ASCII bytes and two-byte hex
  escapes (e.g., `\x1f`), with no surrounding quotes.  Backslash must
  be escaped with itself (e.g., `\\`).

Note the text-format of transactions is subject to change, while the
base-64 XDR version should be backwards compatible.  If you want to
preserve transactions that you can later read or re-use, compile the
transaction with `-c`.  XDR is also compatible with other tools.  You
can also XDR transactions with `stellar-core` itself, using the
command "`stellar-core --base64 --printtxn FILE`", or by using the
web-based Stellar XDR viewer at:
<https://www.stellar.org/laboratory/#xdr-viewer>

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

## Preauth mode

Stellar allows an account to be configured to allow a pre-authorized
transaction with a specific signing weight.  These pre-authorized
transactions are network-dependent hash values represented by strkeys
starting with the letter "T".  Running stc with the `-preauth` flag
outputs this strkey to standard output.

Great care must be taken when creating a pre-authorized transaction,
as any mistake will cause the transaction not to run.  In particular,
make sure you have set the sequence number to one more than it will be
at the time you run the transaction, not one more than it is
currently.  (In particular, if the transaction allowing the
pre-authorized transaction uses the same source account, it will
consume a sequence number.)  You should also make sure the transaction
fee is high enough.  You may wish to increase the fee above what is
currently required in case the fee has increased at the time you need
to execute the pre-authorized transaction.

Another potential source of error is that the pre-authorized
transaction hash depends on the network name, so make absolutely sure
the `-net` option is correct when using `-preauth`.

## Key management mode

stc runs in key management mode when one of the following flags is
selected:  `-keygen`, `-sec2pub`, `import-key`, `-export-key`, and
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

# OPTIONS

`-c`
:	Compile the output to base64 XDR binary.  Otherwise, the default
is to output in text mode.  Only available in default mode.

`-edit`
:	Select edit mode.

`-export-key`
:	Print a private key in strkey format to standard output.

`-help`
:	Print usage information.

`-i`
:	Edit in place--overwrite the input file with the stc's output.  Only
available in default mode.

`-import-key`
: Read a private key from the terminal (or standard input) and write
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

`-list-keys`
:	List all private keys stored under the configuration directory.

`-net` _name_
:	Specify which network to use for hashing, signing, and posting
transactions, as well as for querying signers with the `-l` option.

`-nopass`
:	Never prompt for a passphrase, so assume an empty passphrase
anytime one is required.

`-o` _file_
:	Specify a file in which to write the output.  The default is to
send the transaction to standard output unless `-i` has been
supplied.  `-i` and `-o` are mutually exclusive, and can only be used
in default mode.

`-sec2pub`
:	Print the public key corresponding to a particular private key.

`-sign`
:	Sign the transaction.  If no `-key` option is specified, it will
prompt for the private key on the terminal (or read it from standard
input if standard input is not a terminal).

`-u`
:	Query the network to update the fee and sequence number.  The fee
depends on the number of operations, so be sure to re-run this if you
change the number of transactions.  Only available in default mode.

# EXAMPLES

`stc trans`
:	Reads a transaction from a file called `trans` and prints it to
standard output in human-readable form.

`stc -edit trans`
:	Run the editor on the text format of the transaction in
file `trans` (which can be either text or base64 XDR).  Keep editing
the file until the editor quits without making any changes.

`stc -c -i -key mykey trans`
:	Reads a transaction in file `trans`, signs it using key `mykey`,
then overwrite the `trans` file with the signed transaction in base64
format.

`stc -post trans`
:	Posts a transaction in file `trans` to the network.  The
transaction must previously have been signed.

`stc -keygen`
:	Generate a new private/public key pair, and print them both to
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

Each file in `key` contains a signing key, which is either a single
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
  for the main public Stellar network and test networks.  You probably
  shouldn't edit these files, but may wish to create new ones if you
  instantiate your own networks using the Stellar code base.

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

<https://www.stellar.org/>

# BUGS

stc uses a potentially imperfect heuristic to decide whether a file
contains a base64-encoded binary transaction or a textual one.

Various forms of malformed textual input will surely cause stc to
panic, though the binary parser should be pretty robust.

The tool does not report line numbers for parse errors.

The textual format for transactions is subject to change.  In
particular, it might not make sense to capitalize all field
names--this is mostly an artifact of the go language the tool was
written it.
