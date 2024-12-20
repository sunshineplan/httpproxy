module httpproxy

go 1.23

require (
	github.com/fsnotify/fsnotify v1.8.0
	github.com/sunshineplan/httpproxy v0.0.0-00010101000000-000000000000
	github.com/sunshineplan/limiter v1.0.0
	github.com/sunshineplan/service v1.0.21
	github.com/sunshineplan/utils v0.1.73
	golang.org/x/net v0.33.0
	golang.org/x/time v0.8.0
)

require golang.org/x/sys v0.28.0 // indirect

replace github.com/sunshineplan/httpproxy => ../
