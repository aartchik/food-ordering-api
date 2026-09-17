#!/bin/sh
set -eu

if [ "$#" -ne 2 ]; then
    echo "usage: $0 <api-image> <restaurant-image>" >&2
    exit 2
fi

export API_IMAGE="$1"
export RESTAURANT_IMAGE="$2"
compose="docker compose --project-name food-ordering-api --env-file .env -f deploy/compose.prod.yml"

$compose pull postgres redis migrate
if ! $compose pull api demo-restaurant; then
    docker image inspect "$API_IMAGE" >/dev/null
    docker image inspect "$RESTAURANT_IMAGE" >/dev/null
fi
$compose up -d postgres redis
$compose run --rm migrate
$compose up -d --remove-orphans api demo-restaurant

i=0
until $compose exec -T api wget -qO- http://localhost:8080/readyz >/dev/null; do
    i=$((i + 1))
    if [ "$i" -ge 30 ]; then
        $compose logs --tail=100 api demo-restaurant
        exit 1
    fi
    sleep 2
done

$compose ps
