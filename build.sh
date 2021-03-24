#!/bin/bash

echo "building coin-worker..."
go build -tags=jsoniter -v -o coinservice