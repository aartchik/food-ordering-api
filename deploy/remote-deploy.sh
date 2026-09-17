#!/bin/sh
set -eu

if [ "$#" -ne 2 ]; then
    echo "usage: $0 <api-image> <restaurant-image>" >&2
    exit 2
fi

deploy_dir=/home/deploy/apps/food-ordering-api
mkdir -p "$deploy_dir"
tar -xzf /tmp/food-ordering-api-deploy.tar.gz -C "$deploy_dir"
rm -f /tmp/food-ordering-api-deploy.tar.gz
cd "$deploy_dir"
chmod +x deploy/deploy.sh

docker network inspect public_proxy >/dev/null 2>&1 || docker network create public_proxy >/dev/null

if [ ! -f .env ]; then
    echo "$deploy_dir/.env is missing; production secrets must be created before deployment" >&2
    exit 1
fi

exec ./deploy/deploy.sh "$1" "$2"
