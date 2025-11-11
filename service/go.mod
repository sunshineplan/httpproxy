module httpproxy

go 1.24.0

require (
	github.com/fsnotify/fsnotify v1.9.0
	github.com/sunshineplan/httpproxy v0.0.0-00010101000000-000000000000
	github.com/sunshineplan/limiter v1.0.0
	github.com/sunshineplan/service v1.0.24
	github.com/sunshineplan/utils v0.1.82
	golang.org/x/net v0.47.0
	golang.org/x/time v0.14.0
)

require golang.org/x/sys v0.38.0 // indirect

replace github.com/sunshineplan/httpproxy => ../
