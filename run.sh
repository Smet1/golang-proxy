ECHO 'starting mongo in background'
ECHO 'config:'
cat config.yaml


docker-compose up -d --build "$@"

go run cmd/proxy/main.go