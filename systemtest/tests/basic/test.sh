#!/bin/sh

set -x

# Dump shell environment for context
env

# Print relevant package info
rpm -qi rhc
rpm -qi rhc-worker-playbook
