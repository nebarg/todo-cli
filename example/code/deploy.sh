#!/bin/sh
# todo000 the deploy key is in the repository; rotate it
set -e

make build # todo5 build for every platform here too
rsync -a build/ server:/srv/shop/
