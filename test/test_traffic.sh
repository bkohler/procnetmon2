#!/bin/bash

# Generate some network traffic using netcat
while true; do
    echo "GET / HTTP/1.1
Host: example.com
Connection: close

" | nc example.com 80
    sleep 1
done