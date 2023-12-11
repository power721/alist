docker buildx create --use --platform=linux/arm64,linux/amd64 --name multi-platform-builder
docker buildx inspect --bootstrap
docker buildx build --platform=linux/arm64,linux/amd64 --push --tag haroldli/alist:latest -f Dockerfile .
docker buildx build --platform=linux/arm64,linux/amd64 --push --tag haroldli/alist:hostmode -f Dockerfile-host .
