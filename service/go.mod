module httpproxy

go 1.23.0

toolchain go1.24.0

require (
	github.com/fsnotify/fsnotify v1.8.0
	github.com/sunshineplan/httpproxy v0.0.0-00010101000000-000000000000
	github.com/sunshineplan/limiter v1.0.0
	github.com/sunshineplan/service v1.0.21
	github.com/sunshineplan/utils v0.1.74
	golang.org/x/net v0.36.0
	golang.org/x/time v0.10.0
)

require golang.org/x/sys v0.30.0 // indirect

replace github.com/sunshineplan/httpproxy => ../
