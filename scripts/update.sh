#!/bin/sh
set -e

GOARCH=amd64 GOOS=linux go build main.go     
zip lambda.zip main startpage-template.html feeds.txt  
aws lambda update-function-code --function-name startpage --zip-file fileb://lambda.zip