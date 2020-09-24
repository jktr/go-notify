# go-notify

[![GoDoc](https://godoc.org/github.com/jktr/go-notify?status.svg)](https://godoc.org/github.com/jktr/go-notify)

`go-notify` is a go client library for the [freedesktop.org desktop notification spec](https://specifications.freedesktop.org/notification-spec/latest/index.html
).  
You'll probably mainly want to use it to send desktop notifications over dbus.

It's based on [godbus](https://github.com/godbus/dbus) internally,
and serves a similar role to the familiar `libnotify` and `notify-send`.

Download via `$ go get -u github.com/jktr/go-notify`.

## Getting Started

Take a look at the [example](_example/main.go).  
You gun it with `$ go run ./main.go`.

# Fork

This is a fork of [esiqveland](https://github.com/esiqveland)'s [notify](https://github.com/esiqveland/notify).

Major changes are:
  - api support for `urgency` and inline `image-data` hints
  - notification timeouts now use time.Duration
  - some refactoring for ease of use and api consistency
  - removed embedded logger for easier caller-controlled logging

Some portions of this can probably be upstreamed.

## License

GPLv3
