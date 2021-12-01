#!/bin/sh

S3URL=storage:9000

until curl -i -s $S3URL >/dev/null
do
  echo "Waiting for S3 service at $S3URL."
  sleep 1
done
  
echo "S3 service is available."
exec "$@"