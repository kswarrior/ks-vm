#!/bin/bash
go build -o ksvm .
./ksvm deploy c1 docker://busybox
./ksvm deploy c2 docker://alpine
./ksvm launch c1
./ksvm launch c2
./ksvm list
./ksvm info c1
./ksvm delete c1
./ksvm delete c2
rm ksvm
