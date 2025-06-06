bash build.sh release linux_musl

cp ./build/alist-linux-musl-amd64 /opt/alist/alist
echo "list /opt/alist/alist"
ls -l /opt/alist/alist
