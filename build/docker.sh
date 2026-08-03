docker context create dind

docker buildx create \
    --driver docker-container \
    --name builder \
    --use \
    dindπ

docker buildx inspect --bootstrap

docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t git.pptie.de/pptide/devzat-discord:latest \
  --push .