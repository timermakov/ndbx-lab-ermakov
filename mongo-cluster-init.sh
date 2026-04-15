#!/bin/bash
set -eu

retry() {
  attempts="$1"
  shift
  i=1
  while ! "$@"; do
    if [ "$i" -ge "$attempts" ]; then
      return 1
    fi
    i=$((i + 1))
    sleep 2
  done
}

mongo_eval() {
  host="$1"
  port="$2"
  script="$3"
  mongosh --host "$host" --port "$port" --quiet --eval "$script"
}

retry 30 mongo_eval "mongo-cfg1" "${MONGODB_CONFIG1_PORT}" "db.adminCommand({ ping: 1 })" >/dev/null
retry 30 mongo_eval "mongo-cfg2" "${MONGODB_CONFIG2_PORT}" "db.adminCommand({ ping: 1 })" >/dev/null
retry 30 mongo_eval "mongo-cfg3" "${MONGODB_CONFIG3_PORT}" "db.adminCommand({ ping: 1 })" >/dev/null

retry 30 mongo_eval "mongo-cfg1" "${MONGODB_CONFIG1_PORT}" "
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
" >/dev/null

retry 30 mongo_eval "mongo-cfg1" "${MONGODB_CONFIG1_PORT}" "rs.status().ok" >/dev/null

retry 30 mongo_eval "mongo-shard1-1" "${MONGODB_SHARD1_NODE1_PORT}" "db.adminCommand({ ping: 1 })" >/dev/null
retry 30 mongo_eval "mongo-shard2-1" "${MONGODB_SHARD2_NODE1_PORT}" "db.adminCommand({ ping: 1 })" >/dev/null

retry 30 mongo_eval "mongo-shard1-1" "${MONGODB_SHARD1_NODE1_PORT}" "
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
" >/dev/null

retry 30 mongo_eval "mongo-shard2-1" "${MONGODB_SHARD2_NODE1_PORT}" "
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
" >/dev/null

retry 30 mongo_eval "mongo-shard1-1" "${MONGODB_SHARD1_NODE1_PORT}" "rs.status().ok" >/dev/null
retry 30 mongo_eval "mongo-shard2-1" "${MONGODB_SHARD2_NODE1_PORT}" "rs.status().ok" >/dev/null

retry 30 mongo_eval "mongos" "${MONGODB_PORT}" "db.adminCommand({ ping: 1 })" >/dev/null

retry 30 mongo_eval "mongos" "${MONGODB_PORT}" "
const shards = db.adminCommand({ listShards: 1 }).shards || [];
const hasShard1 = shards.some((s) => s._id === '${MONGODB_SHARD1_NAME}');
const hasShard2 = shards.some((s) => s._id === '${MONGODB_SHARD2_NAME}');

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
" >/dev/null

echo "mongo cluster initialized"
