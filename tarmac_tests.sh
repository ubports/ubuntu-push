#!/bin/bash
# For running tests in Jenkins by Tarmac
set -e
make bootstrap && make check
make check-format
