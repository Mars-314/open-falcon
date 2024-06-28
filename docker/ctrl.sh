#!/bin/sh

DOCKER_DIR=/open-falcon
of_bin=$DOCKER_DIR/open-falcon
DOCKER_HOST_IP=$(route -n | awk '/UG[ \t]/{print $2}')

#use absolute path of metric_list_file in docker
TAB=$'\t'; sed -i "s|.*metric_list_file.*|${TAB}\"metric_list_file\": \"$DOCKER_DIR/api/data/metric\",|g" $DOCKER_DIR/api/config/*.json;

action=$1
module_name=$2
case $action in
 run)
        supervisorctl start "$module_name"
        ;;
 *)
        supervisorctl $*
        ;;
esac
