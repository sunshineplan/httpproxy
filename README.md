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

### Common Command

```
  --host <string>
    	Listening host
  --port <number>
    	Listening port
  --mode <string>
    	Specify server or client mode (default: server)
  --access-log <file>
    	Path to access log file
  --error-log <file>
    	Path to error log file
  --update <url>
    	Update URL
```

### Server Command

```
  --https
    	Serve as HTTPS proxy server
  --cert <file>
    	Path to certificate file
  --privkey <file>
    	Path to private key file
  --secrets <file>
    	Path to secrets file for Basic Authentication
  --whitelist <file>
    	Path to whitelist file
  --status <file>
    	Path to status file
  --keep number
    	Count of status files (default: 100)
```

### Client Command

```
  --proxy <string>
    	Proxy address
  --username <string>
    	Username for Basic Authentication
  --password <string>
    	Password for Basic Authentication
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

### config.ini(server)

```
mode       = server
host       = 0.0.0.0
port       = 443
https      = true
cert       = cert.pem
privkey    = privkey.pem
access-log = /var/log/httpproxy/access.log
error-log  = /var/log/httpproxy/error.log
```

### config.ini(client)

```
mode     = client
port     = 1080
proxy    = https://proxy:443
username = proxy
password = proxy
```

### whitelist

```
8.8.8.8
10.0.0.0/8
```

### secrets

```
user1:password1
user2:password2
```
