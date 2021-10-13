#! /bin/bash

installHTTPProxy() {
    mkdir -p /etc/httpproxy
    curl -Lo- https://github.com/sunshineplan/httpproxy/releases/latest/download/release-linux.tar.gz | tar zxC /etc/httpproxy
    cd /etc/httpproxy
    chmod +x httpproxy
}

configHTTPProxy() {
    touch /etc/httpproxy/secrets
    read -p 'Please enter host (default: 0.0.0.0): ' host
    [ -z $host ] && host=0.0.0.0
    read -p 'Please enter port (default: 8080): ' port
    [ -z $port ] && port=8080
    read -p 'Please enter secrets path: ' secrets
    while true
    do
        read -p 'HTTPS Proxy (default: false): ' https
        [ -z $https ] && https=false && break
        [ $https = true -o $https = false ] && break
        echo https must be true or false!
    done
    read -p 'Please enter cert path: ' cert
    read -p 'Please enter privkey path: ' privkey
    read -p 'Please enter access log path (default: /var/log/httpproxy/access.log): ' access
    [ -z $access ] && access=/var/log/httpproxy/access.log
    read -p 'Please enter error log path (default: /var/log/httpproxy/error.log): ' error
    [ -z $error ] && error=/var/log/httpproxy/error.log
    sed "s/\$host/$host/" /etc/httpproxy/config.ini.default > /etc/httpproxy/config.ini
    sed -i "s/\$port/$port/" /etc/httpproxy/config.ini
    sed -i "s,\$secrets,$secrets," /etc/httpproxy/config.ini
    sed -i "s/\$https/$https/" /etc/httpproxy/config.ini
    sed -i "s,\$cert,$cert," /etc/httpproxy/config.ini
    sed -i "s,\$privkey,$privkey," /etc/httpproxy/config.ini
    sed -i "s,\$access,$access," /etc/httpproxy/config.ini
    sed -i "s,\$error,$error," /etc/httpproxy/config.ini
}

configSysctl() {
    cat >/etc/sysctl.d/90-tcp-keepalive-sysctl.conf <<-EOF
		net.ipv4.tcp_keepalive_time = 600
		net.ipv4.tcp_keepalive_intvl = 60
		net.ipv4.tcp_keepalive_probes = 20
		EOF
    sysctl --system
}

writeLogrotateScrip() {
    mkdir -p /var/log/httpproxy
    cat >/etc/logrotate.d/httpproxy <<-EOF
		/var/log/httpproxy/access.log {
		    copytruncate
		    rotate 15
		    daily
		    compress
		    delaycompress
		    missingok
		    notifempty
		}

		/var/log/httpproxy/error.log {
		    copytruncate
		    rotate 12
		    monthly
		    compress
		    delaycompress
		    missingok
		    notifempty
		}
		EOF
}

main() {
    installHTTPProxy
    configHTTPProxy
    configSysctl
    writeLogrotateScrip
    ./httpproxy install || exit 1
    service httpproxy start
}

main
