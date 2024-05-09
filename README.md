# go-usegcp

`go-usegcp` is a collection of utilities for writing services that use GCP.
Google's official libraries contain parts for interfacing with their APIs but
often do not go as far as finishing actual use cases. This library provides
higher level packages that should generally be usable as-is to solve a problem.

Standard library interfaces are preferred whenever possible. For example,
middleware always uses `http.Handler` and logging always uses `slog`.

## Packages

- [firebaseauth](./middleware/firebaseauth) - an HTTP middleware to verify and decode
Firebase ID tokens.
