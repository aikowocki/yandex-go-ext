#!/bin/sh
set -eu

exec crond -f -l 2
