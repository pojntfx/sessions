module github.com/pojntfx/sessions

go 1.25.0

tool github.com/dennwc/flatpak-go-mod

require (
	github.com/jwijenbergh/puregotk v0.0.0-20251201161753-28ec1479c381
	github.com/pojntfx/go-gettext v0.3.0
	github.com/rymdport/portal v0.4.3-0.20260218173435-15da1e91be7c
)

require (
	github.com/dennwc/flatpak-go-mod v0.1.1-0.20250809093520-ddf8d84264aa // indirect
	github.com/goccy/go-yaml v1.18.0 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/jwijenbergh/purego v0.0.0-20251017112123-b71757b9ba42 // indirect
	golang.org/x/mod v0.27.0 // indirect
)

replace (
	github.com/jwijenbergh/purego v0.0.0-20251017112123-b71757b9ba42 => github.com/pojntfx/purego v0.0.0-20260128031338-5429b1e47a6d
	github.com/jwijenbergh/puregotk v0.0.0-20251201161753-28ec1479c381 => github.com/pojntfx/puregotk v0.0.0-20260220051800-f8a38f1e0894
)
