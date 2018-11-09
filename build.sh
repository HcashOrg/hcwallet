#!/bin/sh

if $1="d"
rm -rf Gopkg.lock
rm -rf vendor
dep ensure
chmod 777 $GOPATH/src/github.com/HcashOrg/hcOMNI/allbuild.sh
$GOPATH/src/github.com/HcashOrg/hcOMNI/allbuild.sh
go install
echo "deep build over"
then if $1="l"
rm -rf Gopkg.lock
rm -rf vendor
dep ensure
chmod 777 $GOPATH/src/github.com/HcashOrg/hcOMNI/libsCopy.sh
$GOPATH/src/github.com/HcashOrg/hcOMNI/libsCopy.sh
go install
echo "light build over"
then if $1="ll"
chmod 777 $GOPATH/src/github.com/HcashOrg/hcOMNI/libsCopy.sh
$GOPATH/src/github.com/HcashOrg/hcOMNI/libsCopy.sh
go install
echo "light build over"
then
go install
echo "go install over"
fi


