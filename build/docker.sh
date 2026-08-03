docker context create dind

docker buildx create \
    --name builder \
    --driver docker-container \
    --context dind \
    --use

docker buildx inspect --bootstrap

docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t git.pptie.de/pptide/devzat-discord:latest \
  --push .