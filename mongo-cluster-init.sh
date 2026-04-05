#!/bin/bash
set -euo pipefail

wait_for_mongo() {
  local host="$1"
  local port="$2"

  until mongosh --host "$host" --port "$port" --quiet --eval "db.adminCommand({ ping: 1 }).ok" >/dev/null 2>&1; do
    sleep 2
  done
}

init_rs_if_needed() {
  local host="$1"
  local port="$2"
  local js="$3"

  mongosh --host "$host" --port "$port" --quiet --eval "$js"
}

wait_for_mongo "mongo-cfg1" "${MONGODB_CONFIG1_PORT}"
wait_for_mongo "mongo-cfg2" "${MONGODB_CONFIG2_PORT}"
wait_for_mongo "mongo-cfg3" "${MONGODB_CONFIG3_PORT}"
wait_for_mongo "mongo-shard1-1" "${MONGODB_SHARD1_NODE1_PORT}"
wait_for_mongo "mongo-shard1-2" "${MONGODB_SHARD1_NODE2_PORT}"
wait_for_mongo "mongo-shard1-3" "${MONGODB_SHARD1_NODE3_PORT}"
wait_for_mongo "mongo-shard2-1" "${MONGODB_SHARD2_NODE1_PORT}"
wait_for_mongo "mongo-shard2-2" "${MONGODB_SHARD2_NODE2_PORT}"
wait_for_mongo "mongo-shard2-3" "${MONGODB_SHARD2_NODE3_PORT}"

init_rs_if_needed "mongo-cfg1" "${MONGODB_CONFIG1_PORT}" "
try {
  rs.status();
} catch (e) {
  rs.initiate({
    _id: '${MONGODB_CONFIG_REPLICA_SET}',
    configsvr: true,
    members: [
      { _id: 0, host: 'mongo-cfg1:${MONGODB_CONFIG1_PORT}' },
      { _id: 1, host: 'mongo-cfg2:${MONGODB_CONFIG2_PORT}' },
      { _id: 2, host: 'mongo-cfg3:${MONGODB_CONFIG3_PORT}' }
    ]
  });
}
"

init_rs_if_needed "mongo-shard1-1" "${MONGODB_SHARD1_NODE1_PORT}" "
try {
  rs.status();
} catch (e) {
  rs.initiate({
    _id: '${MONGODB_SHARD1_REPLICA_SET}',
    members: [
      { _id: 0, host: 'mongo-shard1-1:${MONGODB_SHARD1_NODE1_PORT}' },
      { _id: 1, host: 'mongo-shard1-2:${MONGODB_SHARD1_NODE2_PORT}' },
      { _id: 2, host: 'mongo-shard1-3:${MONGODB_SHARD1_NODE3_PORT}' }
    ]
  });
}
"

init_rs_if_needed "mongo-shard2-1" "${MONGODB_SHARD2_NODE1_PORT}" "
try {
  rs.status();
} catch (e) {
  rs.initiate({
    _id: '${MONGODB_SHARD2_REPLICA_SET}',
    members: [
      { _id: 0, host: 'mongo-shard2-1:${MONGODB_SHARD2_NODE1_PORT}' },
      { _id: 1, host: 'mongo-shard2-2:${MONGODB_SHARD2_NODE2_PORT}' },
      { _id: 2, host: 'mongo-shard2-3:${MONGODB_SHARD2_NODE3_PORT}' }
    ]
  });
}
"

sleep "${MONGODB_INIT_SLEEP_SECONDS}"
wait_for_mongo "mongos" "${MONGODB_MONGOS_PORT}"

mongosh --host "mongos" --port "${MONGODB_MONGOS_PORT}" --quiet --eval "
const existingShards = db.adminCommand({ listShards: 1 }).shards || [];
const hasShard1 = existingShards.some((shard) => shard._id === '${MONGODB_SHARD1_NAME}');
const hasShard2 = existingShards.some((shard) => shard._id === '${MONGODB_SHARD2_NAME}');

if (!hasShard1) {
  sh.addShard('${MONGODB_SHARD1_REPLICA_SET}/mongo-shard1-1:${MONGODB_SHARD1_NODE1_PORT},mongo-shard1-2:${MONGODB_SHARD1_NODE2_PORT},mongo-shard1-3:${MONGODB_SHARD1_NODE3_PORT}', '${MONGODB_SHARD1_NAME}');
}
if (!hasShard2) {
  sh.addShard('${MONGODB_SHARD2_REPLICA_SET}/mongo-shard2-1:${MONGODB_SHARD2_NODE1_PORT},mongo-shard2-2:${MONGODB_SHARD2_NODE2_PORT},mongo-shard2-3:${MONGODB_SHARD2_NODE3_PORT}', '${MONGODB_SHARD2_NAME}');
}

sh.enableSharding('${MONGODB_DATABASE}');
sh.shardCollection('${MONGODB_DATABASE}.events', { created_by: 'hashed' });

const appDb = db.getSiblingDB('${MONGODB_DATABASE}');
if (!appDb.getUser('${MONGODB_USER}')) {
  appDb.createUser({
    user: '${MONGODB_USER}',
    pwd: '${MONGODB_PASSWORD}',
    roles: [
      { role: 'readWrite', db: '${MONGODB_DATABASE}' },
      { role: 'dbOwner', db: '${MONGODB_DATABASE}' }
    ]
  });
}
"
