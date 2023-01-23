# text-relay

This is an overly simplistic text-relay server written on behalf of a friend as
a challenge.

## The Goal

This is meant to help handle the following situation:

```
Someone uses a cellphone to start their business. At some point, they decide to
port their number over to use a VoIP solution. Their old cell number is now
their main work line. This number may still receive text messages (for things
like 2fa, etc) from folks that haven't gotten the new personal number yet.
```

## Rules

* A text cannot be sent from a number that isn't controlled by the provider (no spoofing!)
* A postfix should be added with a reference to the original number
* The text should be sent to a new arbitrary number
* Texts from numbers not listed in `relays.csv` will be ignored

## Flow

* The original number receives a text to some VoIP stack
* The stack will issue an HTTP POST to a specified url with the text details
* The url is handled by the `text-relay` service
* The payload is re-written using the `relays.csv` data file
* The service sends the new payload to a text messaging service, forwarding the text

## Data

The actual relays are stored in `relays.csv`, colocated with the binary. There
is minimal validation on the actual numbers. They must be in whatever format
the text messaging service expects.

The first column is the original destination (and thus, the new originator).
The second column is the target to receive the message.

## Building

This application is written in Go. You'll need a relatively recent installation
of Go. It must at least support modules. Versions other than `1.19.1` have not
been tested.

To test locally:

```bash
go build
```

The resulting binary should be named `text-relay` and can run locally.

To build for running on a linux server:

```bash
GOOS=linux GOARCH=amd64 go build -o text-relay-linux
```

Substitute whatever `GOARCH` or `GOOS` matches your target system.

## Running

The application expects the `relays.csv` to be colocated with the binary. It
will fatally error should this not be true.

To run and configure, use the output below as a guide.

```bash
$ PORT=9001 TOKEN=<API TOKEN> ACCOUNT_ID=<ACCOUNT ID> ./text-relay
{"level":"debug","time":1674512034,"message":"starting text-relay on port 9001"}
```

Set the `TOKEN` and `ACCOUNT_ID` as required to communicate with the text
messaging service. Set the `PORT` that the service should be exposed on. Once
started, it will start logging to standard out in JSON.
