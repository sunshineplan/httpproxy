# HTTP(S) Proxy

A Standard HTTP(S) proxy server.

If secrets file is changed, it will be reloaded automatically.

## Installation

```bash
curl -Lo- https://github.com/sunshineplan/httpproxy/releases/latest/download/release-linux.tar.gz | tar zxC .
chmod +x httpproxy
./httpproxy install
./httpproxy start
```
You can also build your own binary by:
```cmd
git clone https://github.com/sunshineplan/httpproxy.git
cd httpproxy
go build
```

## Usage

### Command Line

```
  --host <string>
    	Listening host
  --port <number>
    	Listening port
  --secrets <file>
    	Path to secrets file
  --https
    	Serve as HTTPS proxy server
  --cert <file>
    	Path to certificate file
  --privkey <file>
    	Path to private key file
  --access-log <file>
    	Path to access log file
  --error-log <file>
    	Path to error log file
  --update <url>
    	Update URL
```

### Service Command

```
  install
    	Install service
  uninstall/remove
    	Uninstall service
  run
    	Run service executor
  test
    	Run service test executor	
  start
    	Start service
  stop
    	Stop service
  restart
    	Restart service
  update
    	Update service files if update url is provided
```

## Example config

### config.ini

```
host       = 0.0.0.0
port       = 443
https      = true
cert       = cert.pem
privkey    = privkey.pem
access-log = /var/log/httpproxy/access.log
error-log  = /var/log/httpproxy/error.log
```

### secrets

```
user1:password1
user2:password2
```