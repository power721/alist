bash build.sh release linux_musl
ls -l build
sudo cp ./build/alist-linux-musl-amd64 /opt/atv/alist/alist
sudo chown $USER /opt/atv/alist/alist
ls -l /opt/atv/alist/alist
