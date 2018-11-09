#!/bin/sh

if [ "$1" = "d" ]; then
    rm -rf Gopkg.lock
    rm -rf vendor
    dep ensure
    cd $GOPATH/src/github.com/HcashOrg/hcOMNI/
    #chmod 777 $GOPATH/src/github.com/HcashOrg/hcOMNI/allbuild.sh
    chmod 777 allbuild.sh
    ./allbuild.sh
    cd -
    go install
    echo "deep build over"
elif [ "$1" = "l" ]; then
    rm -rf Gopkg.lock
    rm -rf vendor
    dep ensure
    cd $GOPATH/src/github.com/HcashOrg/hcOMNI/
    chmod 777 libsCopy.sh
    ./libsCopy.sh
    cd -
    go install
    echo "light build over"
elif [ "$1" = "ld" ]; then
    cd $GOPATH/src/github.com/HcashOrg/hcOMNI/
    chmod 777 allbuild.sh
    ./allbuild.sh
    cd -
    go install
    echo "light build over ld"
elif [ "$1" = "ll" ]; then
    cd $GOPATH/src/github.com/HcashOrg/hcOMNI/
    chmod 777 libsCopy.sh
    ./libsCopy.sh
    cd -
    go install
    echo "light build over ll"
else
    go install
    echo "go install over"
fi
