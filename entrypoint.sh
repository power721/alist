#!/bin/sh
set -e

[ -n "$UPDATE_URL" ] || UPDATE_URL=http://docker.xiaoya.pro

if [ ! -f /opt/alist/data/data.db ]; then
	echo "Download $UPDATE_URL/data.db"
	wget -O /opt/alist/data/data.db $UPDATE_URL/data.db
	mkdir -p /opt/alist/data/www
fi

/updatedb

if [ -n "$ALI_TOKEN" ]; then
	/token $ALI_TOKEN
	echo `date` "User's own token $ALI_TOKEN has been updated into database succefully"
elif [[ -f /opt/alist/data/mytoken.txt ]] && [[ -s /opt/alist/data/mytoken.txt ]]; then
	user_token=$(head -n1 /opt/alist/data/mytoken.txt)
        /token $user_token
	echo `date` "User's own token $user_token has been updated into database succefully"
fi

cd /opt/alist
/bin/busybox-extras httpd -p 81 -h /www
/usr/sbin/nginx
/updateindex

exec "$@"
