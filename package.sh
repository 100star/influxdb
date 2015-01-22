#!/bin/bash

INSTALL_ROOT_DIR=/opt/influxdb

TMP_WORK_DIR=`mktemp -d`
POST_INSTALL_PATH=`mktemp`
ARCH=`uname -i`

###########################################################################
# Helper functions.
###########################################################################

# usage prints simple usage information.
usage() {
    echo -e "$0 [<version>] [-h]\n"
    cleanup_exit $1
}

# cleanup_exit removes all resources created during the process and exits with
# the supplied returned code.
cleanup_exit() {
    rm -r $TMP_WORK_DIR
    rm $POST_INSTALL_PATH
    exit $1
}

# check_gopath sanity checks the value of the GOPATH env variable.
check_gopath() {
    [ -z "$GOPATH" ] && echo "GOPATH is not set." && cleanup_exit 1
    [ ! -d "$GOPATH" ] && echo "GOPATH is not a directory." && cleanup_exit 1
    echo "GOPATH ($GOPATH) looks sane."
}

# check_clean_tree ensures that no source file is locally modified.
check_clean_tree() {
    modified=$(git ls-files --modified | wc -l)
    if [ $modified -ne 0 ]; then
        echo "The source tree is not clean -- aborting."
        cleanup_exit 1
    fi
    echo "Git tree is clean."
}

# update_tree ensures the tree is in-sync with the repo.
update_tree() {
    git pull origin master
    if [ $? -ne 0 ]; then
        echo "Failed to pull latest code -- aborting."
        cleanup_exit 1
    fi
    git fetch --tags
    if [ $? -ne 0 ]; then
        echo "Failed to fetch tags -- aborting."
        cleanup_exit 1
    fi
    echo "Git tree updated successfully."
}

# do_build builds the code.
do_build() {
    rm $GOPATH/bin/*
    go install -a ./...
    if [ $? -ne 0 ]; then
        echo "Build failed, unable to create package -- aborting"
        cleanup_exit 1
    fi
    echo "Build completed successfully."
}

# generate_postinstall_script creates the post-install script for the
# package. It must be passed the version.
generate_postinstall_script() {
    version=$1
    cat  <<EOF >$POST_INSTALL_PATH
rm $INSTALL_ROOT_DIR/influxd
ln -s  $INSTALL_ROOT_DIR/versions/$version/influxd $INSTALL_ROOT_DIR/influxd

if ! id influxdb >/dev/null 2>&1; then
        useradd --system -U -M influxdb
fi
chown -R -L influxdb:influxdb $INFLUXDB_DIR
chmod -R a+rX $INFLUXDB_DIR
EOF
    echo "Post-install script created successfully at $POST_INSTALL_PATH"
}

###########################################################################
# Start the packaging process.
###########################################################################

if [ $# -ne 1 ]; then
    usage 1
elif [ $1 == "-h" ]; then
    usage 0
else
    VERSION=$1
fi

echo -e "\nStarting package process...\n"

check_gopath
check_clean_tree
update_tree
do_build

mkdir -p $TMP_WORK_DIR/$INSTALL_ROOT_DIR/versions/$VERSION
if [ $? -ne 0 ]; then
    echo "Failed to create temporary work directory for packaging -- aborting."
    cleanup_exit 1
fi
echo "Temporary work directory created in $TMP_WORK_DIR"

cp $GOPATH/bin/* $TMP_WORK_DIR/$INSTALL_ROOT_DIR/versions/$VERSION
if [ $? -ne 0 ]; then
    echo "Failed to copy binaries to packaging directory -- aborting."
    cleanup_exit 1
fi
echo "Binaries in $GOPATH/bin copied to $TMP_WORK_DIR/$INSTALL_ROOT_DIR/versions/$VERSION"

generate_postinstall_script $VERSION

echo -n "Commence creation of $ARCH packages, version $VERSION? [Y/n] "
read response
response=`echo $response | tr 'A-Z' 'a-z'`
if [ "x$response" == "xn" ]; then
    echo "Packaging aborted."
    cleanup_exit 1
fi

if [ $ARCH == "i386" ]; then
    rpm_package=influxdb-$(VERSION)-1.i686.rpm
    debian_package=influxdb_$(VERSION)_i686.deb
    deb_args="-a i686"
    rpm_args="setarch i686"
elif [ $ARCH == "arm" ]; then
    rpm_package=influxdb-$(VERSION)-1.armel.rpm
    debian_package=influxdb_$(VERSION)_armel.deb
else
    rpm_package=influxdb-$(package_version)-1.x86_64.rpm
    debian_package=influxdb_$(VERSION)_amd64.deb
fi

echo $rpm_args fpm -s dir -t rpm --after-install $POST_INSTALL_PATH -n influxdb -v $VERSION $TMP_WORK_DIR
if [ $? -ne 0 ]; then
    echo "Failed to create RPM package -- aborting"
    cleanup_exit 1
fi

echo fpm -s dir -t deb $deb_args --after-install $POST_INSTALL_PATH -n influxdb -v -v $VERSION $TMP_WORK_DIR
if [ $? -ne 0 ]; then
    echo "Failed to create Debian package -- aborting"
    cleanup_exit 1
fi

echo "Packaging successful."
cleanup_exit 0
