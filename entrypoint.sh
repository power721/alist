#!/bin/sh
set -e

cd /tmp/

if [ -f update.zip ]; then
	rm update.zip
fi

if [ -f update.sql ]; then
        rm update.sql
fi

if [ -f version.txt ]; then
	rm version.txt
fi

echo "Download http://docker.xiaoya.pro/update.zip"
wget -T 5 -q http://docker.xiaoya.pro/update.zip

if [ ! -f update.zip ]; then
        echo "Failed to download updated files, the upgrade process has aborted"
else
	if [ ! -f /opt/alist/data/data.db ]; then
		echo "Download http://docker.xiaoya.pro/data.db"
		wget -O /opt/alist/data/data.db http://docker.xiaoya.pro/data.db
		mkdir -p /opt/alist/data/www
	fi
        unzip -o -q update.zip
	remote=$(head -n1 version.txt)
	entries=$(expr `cat update.sql|wc -l` - 4)
	echo `date` "total" $entries "records added"
        sqlite3 /opt/alist/data/data.db <<EOF
drop table IF EXISTS x_storages;
.read update.sql
EOF
	echo `date` "update database succesfully, now version is " $remote
	echo $remote > /opt/alist/version.txt
	rm update.*
	rm version.txt
fi

if [[ -f /mytoken.txt ]] && [[ -s /mytoken.txt ]]; then
	user_token=$(head -n1 /mytoken.txt)
        /token $user_token
	echo `date` "User's own token $user_token has been updated into database succefully"
fi

cd /opt/alist
/bin/busybox-extras httpd -p 81 -h /www
/usr/sbin/nginx
/updateindex

exec "$@"

