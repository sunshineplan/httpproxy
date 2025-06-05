module httpproxy

go 1.24

require (
	github.com/fsnotify/fsnotify v1.9.0
	github.com/sunshineplan/httpproxy v0.0.0-00010101000000-000000000000
	github.com/sunshineplan/limiter v1.0.0
	github.com/sunshineplan/service v1.0.22
	github.com/sunshineplan/utils v0.1.76
	golang.org/x/net v0.40.0
	golang.org/x/time v0.12.0
)

require golang.org/x/sys v0.33.0 // indirect

replace github.com/sunshineplan/httpproxy => ../
