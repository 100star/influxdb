#!/bin/bash

# Check that GOPATH is set.
# Check tree is clean.
# Build binaries.
# Check expected binaries exist.
# Check all error codes.
# Clean up on exit -- bad or otherwise.
# Let user know at each step, confirm if necessary.

INFLUXDB_DIR=/opt/influxdb

TMPDIR=`mktemp -d`
TMP_POSTINSTALL=`mktemp`

VERSION=`$GOPATH/bin/influxd version 2>&1 | cut -d ' ' -f 2`
if [ -z "$VERSION" ]; then
    exit 1
fi

mkdir -p $TMPDIR/$INFLUXDB_DIR/versions/$VERSION
cp $GOPATH/bin/* $TMPDIR/$INFLUXDB_DIR/versions/$VERSION

cat  <<EOF >$TMP_POSTINSTALL
rm $INFLUXDB_DIR/influxd
ln -s  $INFLUXDB_DIR/versions/$VERSION/influxd $INFLUXDB_DIR/influxd

if ! id influxdb >/dev/null 2>&1; then
        useradd --system -U -M influxdb
fi
chown -R -L influxdb:influxdb $INFLUXDB_DIR
chmod -R a+rX $INFLUXDB_DIR
EOF

fpm -s dir -t rpm --after-install $TMP_POSTINSTALL -n influxdb -v $VERSION $TMPDIR
fpm -s dir -t deb -a i686 --after-install $TMP_POSTINSTALL -n influxdb -v -v $VERSION $TMPDIR

