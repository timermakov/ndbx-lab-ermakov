#!/usr/bin/env bash

set -euo pipefail

cass_home="${CASSANDRA_HOME:-/opt/cassandra}"
if [[ -d "${cass_home}/bin" ]]; then
  PATH="${cass_home}/bin:${PATH}"
  export PATH
fi

if [[ -z "${CASSANDRA_HOSTS:-}" ]]; then
  echo "CASSANDRA_HOSTS is required"
  exit 1
fi

if [[ -z "${CASSANDRA_PORT:-}" ]]; then
  echo "CASSANDRA_PORT is required"
  exit 1
fi

if [[ -z "${CASSANDRA_KEYSPACE:-}" ]]; then
  echo "CASSANDRA_KEYSPACE is required"
  exit 1
fi

if [[ ! "${CASSANDRA_KEYSPACE}" =~ ^[a-zA-Z0-9_]+$ ]]; then
  echo "CASSANDRA_KEYSPACE must contain only [a-zA-Z0-9_]"
  exit 1
fi

cassandra_host="${CASSANDRA_HOSTS%%,*}"
auth_args=()
if [[ -n "${CASSANDRA_USERNAME:-}" && -n "${CASSANDRA_PASSWORD:-}" ]]; then
  auth_args=(-u "${CASSANDRA_USERNAME}" -p "${CASSANDRA_PASSWORD}")
fi

for attempt in $(seq 1 30); do
  # stdin must not be the script pipe (see docker-compose tr|bash); cqlsh would consume the rest of the script.
  if cqlsh "${cassandra_host}" "${CASSANDRA_PORT}" "${auth_args[@]}" -e "DESCRIBE KEYSPACES" >/dev/null 2>&1 </dev/null; then
    break
  fi

  if [[ "${attempt}" == "30" ]]; then
    echo "Cassandra is not ready after ${attempt} attempts"
    exit 1
  fi

  sleep 2
done

tr -d '\r' < /scripts/init.cql | sed "s/__CASSANDRA_KEYSPACE__/${CASSANDRA_KEYSPACE}/g" > /tmp/cassandra-init.cql
cqlsh "${cassandra_host}" "${CASSANDRA_PORT}" "${auth_args[@]}" -f /tmp/cassandra-init.cql </dev/null
