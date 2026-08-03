docker buildx create --driver docker-container --use --name builder || docker buildx use builder
docker buildx inspect --bootstrap

docker login git.pptie.de -u "$REGISTRY_USER" -p "$REGISTRY_PASSWORD"

docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t git.pptie.de/pptide/devzat-discord:latest \
  --push .