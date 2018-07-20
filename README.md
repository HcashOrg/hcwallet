hcwallet
=========

[![ISC License](http://img.shields.io/badge/license-ISC-blue.svg)](http://copyfree.org)

hcwallet is a daemon handling hc wallet functionality for a
single user.  It acts as both an RPC client to hcd and an RPC server
for wallet clients and legacy RPC applications.

Public and private keys are derived using the hierarchical
deterministic format described by
[BIP0032](https://github.com/bitcoin/bips/blob/master/bip-0032.mediawiki).
Unencrypted private keys are not supported and are never written to
disk.  hcwallet uses the
`m/44'/<coin type>'/<account>'/<branch>/<address index>`
HD path for all derived addresses, as described by
[BIP0044](https://github.com/bitcoin/bips/blob/master/bip-0044.mediawiki).

Due to the sensitive nature of public data in a BIP0032 wallet,
hcwallet provides the option of encrypting not just private keys, but
public data as well.  This is intended to thwart privacy risks where a
wallet file is compromised without exposing all current and future
addresses (public keys) managed by the wallet. While access to this
information would not allow an attacker to spend or steal coins, it
does mean they could track all transactions involving your addresses
and therefore know your exact balance.  In a future release, public data
encryption will extend to transactions as well.

hcwallet is not an SPV client and requires connecting to a local or
remote hcd instance for asynchronous blockchain queries and
notifications over websockets.  Full hcd installation instructions
can be found [here](https://github.com/HcashOrg/hcd).  An alternative
SPV mode that is compatible with hcd is planned for a future release.

Wallet clients can use one of two RPC servers:

  1. A legacy JSON-RPC server inspired by the Bitcoin Core rpc server

     The JSON-RPC server exists to ease the migration of wallet applications
     from Core, but complete compatibility is not guaranteed.  Some portions of
     the API (and especially accounts) have to work differently due to other
     design decisions (mostly due to BIP0044).  However, if you find a
     compatibility issue and feel that it could be reasonably supported, please
     report an issue.  This server is enabled by default as long as a username
     and password are provided.

  2. A gRPC server

     The gRPC server uses a new API built for hcwallet, but the API is not
     stabilized.  This server is enabled by default and may be disabled with
     the config option `--nogrpc`.  If you don't mind applications breaking
     due to API changes, don't want to deal with issues of the legacy API, or
     need notifications for changes to the wallet, this is the RPC server to
     use. The gRPC server is documented [here](./rpc/documentation/README.md).

## Current State

This project is currently under active development and is in a Beta state. The default branch of hcwallet is currently testnet1. Please make sure to use --testnet flag when running hcwallet and report any issues by using the integrated issue tracker. Do not yet use this software yet as a store of value!

MAKE SURE TO SAVE YOUR WALLET SEED SO YOU CAN RECOVER YOUR KEYS IF THE WALLET FAILS!

## Installation and updating

### Windows/Linux/BSD/POSIX - Build from source

Building or updating from source requires the following build dependencies:

- **Go 1.8 or 1.9**

  Installation instructions can be found here: http://golang.org/doc/install.
  It is recommended to add `$GOPATH/bin` to your `PATH` at this point.

- **Dep**

  Dep is used to manage project dependencies and provide reproducible builds.
  It is recommended to use the latest Dep release, unless a bug prevents doing
  so.  The latest releases (for both binary and source) can be found
  [here](https://github.com/golang/dep/releases).

Unfortunately, the use of `dep` prevents a handy tool such as `go get` from
automatically downloading, building, and installing the source in a single
command.  Instead, the latest project and dependency sources must be first
obtained manually with `git` and `dep`, and then `go` is used to build and
install the project.

**Getting the source**:

For a first time installation, the project and dependency sources can be
obtained manually with `git` and `dep` (create directories as needed):

```
git clone https://github.com/HcashOrg/hcwallet $GOPATH/src/github.com/HcashOrg/hcwallet
cd $GOPATH/src/github.com/HcashOrg/hcwallet
dep ensure
go install
```

To update an existing source tree, pull the latest changes and install the
matching dependencies:

```
cd $GOPATH/src/github.com/HcashOrg/hcwallet
git pull
dep ensure -update
go install
```

**Building/Installing**:

The `go` tool is used to build or install (to `GOPATH`) the project.  Some
example build instructions are provided below (all must run from the `hcwallet`
project directory).

To build and install `hcwallet` and all helper commands (in the `cmd`
directory) to `$GOPATH/bin/`, as well as installing all compiled packages to
`$GOPATH/pkg/` (**use this if you are unsure which command to run**):

```
go install . ./cmd/...
```

To build a `hcwallet` executable and install it to `$GOPATH/bin/`:

```
go install
```

To build a `hcwallet` executable and place it in the current directory:

```
go build
```

## Getting Started

The following instructions detail how to get started with hcwallet connecting
to a localhost hcd.  Commands should be run in `cmd.exe` or PowerShell on
Windows, or any terminal emulator on *nix.

- Run the following command to start hcd:

```
hcd -u rpcuser -P rpcpass --testnet
```

- Run the following command to create a wallet:

```
hcwallet -u rpcuser -P rpcpass --testnet --create
```

- Run the following command to start hcwallet:

```
hcwallet -u rpcuser -P rpcpass --testnet
```

If everything appears to be working, it is recommended at this point to
copy the sample hcd and hcwallet configurations and update with your
RPC username and password.

PowerShell (Installed from MSI):
```
PS> cp "$env:ProgramFiles\HcashOrg\Hcd\sample-hcd.conf" $env:LOCALAPPDATA\Hcd\hcd.conf
PS> cp "$env:ProgramFiles\HcashOrg\hcwallet\sample-hcwallet.conf" $env:LOCALAPPDATA\hcwallet\hcwallet.conf
PS> $editor $env:LOCALAPPDATA\Hcd\hcd.conf
PS> $editor $env:LOCALAPPDATA\hcwallet\hcwallet.conf
```

PowerShell (Installed from source):
```
PS> cp $env:GOPATH\src\github.com\HcashOrg\hcd\sample-hcd.conf $env:LOCALAPPDATA\Hcd\hcd.conf
PS> cp $env:GOPATH\src\github.com\HcashOrg\hcwallet\sample-hcwallet.conf $env:LOCALAPPDATA\Hcwallet\hcwallet.conf
PS> $editor $env:LOCALAPPDATA\Hcd\hcd.conf
PS> $editor $env:LOCALAPPDATA\Hcwallet\hcwallet.conf
```

Linux/BSD/POSIX (Installed from source):
```bash
$ cp $GOPATH/src/github.com/HcashOrg/hcd/sample-hcd.conf ~/.hcd/hcd.conf
$ cp $GOPATH/src/github.com/HcashOrg/hcwallet/sample-hcwallet.conf ~/.hcwallet/hcwallet.conf
$ $EDITOR ~/.hcd/hcd.conf
$ $EDITOR ~/.hcwallet/hcwallet.conf
```

## Issue Tracker

The [integrated github issue tracker](https://github.com/HcashOrg/hcwallet/issues)
is used for this project.

## License

hcwallet is licensed under the liberal ISC License.
