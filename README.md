# Nos Event Service

A service which crawls relays to replicate events relevant to Nos users which
then get passed to other services e.g. the notification service or the
moderation service.

## Design

The service connects to all known relays and replicates the following events:
- all events of kinds defined in [`globalEventTypesToDownload`][global-event-kinds-to-download-search]
- all events created by Nos users or users in their contacts lists

The relays are discovered by using the code located in
[`RelaysExtractor`][relays-extractor-search] to scan nostr events. There is also
a set of  bootstrap relays returned by
[`BootstrapRelaySource`][bootstrap-relay-source-search].

The contacts are discovered by using the code located in
[`ContactsExtractor`][contacts-extractor-search] to scan nostr events.


## Building and running

Build the program like so:

    $ go build -o event-service ./cmd/event-service
    $ ./event-service

The program takes no arguments. There is a Dockerfile available.

## Configuration

Configuration is performed using environment variables. This is also the case
for the Dockerfile.

### `EVENTS_LISTEN_ADDRESS`

Listen address for the main webserver in the format accepted by the Go standard
library. The metrics are exposed under path `/metrics`.

Optional, defaults to `:8008` if empty.

### `EVENTS_ENVIRONMENT`

Execution environment. Setting environment to `DEVELOPMENT`:
- turns off posting events to Google PubSub

Optional, can be set to `PRODUCTION` or `DEVELOPMENT`. Defaults to `PRODUCTION`.

### `EVENTS_LOG_LEVEL`

Log level.

Optional, can be set to `TRACE`, `DEBUG`, `ERROR` or `DISABLED`. Defaults to
`DEBUG`.

### `EVENTS_DATABASE_PATH`

Full path to the database file.

Required, e.g. `/some/directory/database.sqlite`.

### `EVENTS_GOOGLE_PUBSUB_PROJECT_ID`

Project ID used for Google Cloud Pubsub. Required when `EVENTS_ENVIRONMENT` is
set to `PRODUCTION`.

### `EVENTS_GOOGLE_PUBSUB_CREDENTIALS_JSON_PATH`

Path to your Google Cloud credentials JSON file. Required when
`EVENTS_ENVIRONMENT` is set to `PRODUCTION`.

### `EVENTS_PYROSCOPE_APPLICATION_NAME`

Pyroscope application name.

Optional, defaults to `events.nos.social`. Relevant only if Pyroscope server
address is set.

### `EVENTS_PYROSCOPE_SERVER_ADDRESS`

Pyroscope server address. If this variable is set to an empty string then
uploading metrics to a Pyroscope server is disabled. This should be a full
address e.g. `https://example.com`.

Optional, defaults to an empty string disabling pushing Pyroscope metrics.

### `PYROSCOPE_BASIC_AUTH_USER`

User component of basic auth of the Pyroscope server.

Optional, disabled by default. Relevant only if Pyroscope server address is set.

### `PYROSCOPE_BASIC_AUTH_PASSWORD`

Password component of basic auth of the Pyroscope server.

Optional, disabled by default. Relevant only if Pyroscope server address is set.

## Metrics

See configuration for the address of our metrics endpoint. Many out-of-the-box
Go-related metrics are available. We also have custom metrics:

- `application_handler_calls_total`
- `application_handler_calls_duration`
- `version`
- `subscription_queue_length`
- `relay_downloader_gauge`
- `relay_connection_state_gauge`
- `received_events_counter`
- `relay_connection_subscriptions_gauge`
- `relay_connection_received_messages_counter`
- `relay_connection_reconnections_counter`
- `stored_relay_addresses_gauge`

See `service/adapters/prometheus`.

## Contributing

### Go version

The project usually uses the latest Go version as declared by the `go.mod` file.
You may not be able to build it using older compilers.

### How to do local development

Run the following command changing appropriate environment variables:

```
EVENTS_DATABASE_PATH=/path/to/database.sqlite \
EVENTS_ENVIRONMENT=DEVELOPMENT \
go run ./cmd/event-service
```

### Updating frontend files

Frontend is written in Vue and located in `./frontend`. Precompiled files are
supposed to be commited as they are embedded in executable files.

In order to update the embedded compiled frontend files run the following
command:

    $ make frontend

### Makefile

We recommend reading the `Makefile` to discover some targets which you can
execute. It can be used as a shortcut to run various useful commands.

You may have to run the following command to install a linter and a code
formatter before executing certain targets:

    $ make tools

If you want to check if the pipeline will pass for your commit it should be
enough to run the following command:

    $ make ci

It is also useful to often run just the tests during development:

    $ make test

Easily format your code with the following command:

    $ make fmt

### Writing code

Resources which are in my opinion informative and good to read:

- [Effective Go][effective-go]
- [Go Code Review Comments][code-review-comments]
- [Uber Go Style Guide][uber-style-guide]

#### Naming tests

When naming tests which tests a specific behaviour it is recommended to follow a
pattern `TestNameOfType_ExpectedBehaviour`. Example:
`TestRelayDownloader_EventsDownloadedFromRelaysArePublishedUsingPublisher`
.

#### Panicking constructors

Some constructors are prefixed with the word `Must`. Those constructors panic
and should always be accompanied by a normal constructor which isn't prefixed
with the `Must` and returns an error. The panicking constructors should only be
used in the following cases:
- when writing tests
- when a static value has to be created e.g. `MustNewHops(1)` and this branch of
  logic in the code is covered by tests

[effective-go]: http://golang.org/doc/effective_go.html
[code-review-comments]: https://github.com/golang/go/wiki/CodeReviewComments
[uber-style-guide]: https://github.com/uber-go/guide/blob/master/style.md

[global-event-kinds-to-download-search]: https://github.com/search?q=repo%3Aplanetary-social%2Fnos-event-service+globalEventKindsToDownload&type=code
[relays-extractor-search]: https://github.com/search?q=repo%3Aplanetary-social%2Fnos-event-service+RelaysExtractor&type=code
[contacts-extractor-search]: https://github.com/search?q=repo%3Aplanetary-social%2Fnos-event-service+ContactsExtractor&type=code
[bootstrap-relay-source-search]: https://github.com/search?q=repo%3Aplanetary-social%2Fnos-event-service%20BootstrapRelaySource&type=code