#!/bin/bash -

python -m grpc_tools.protoc --proto_path=../yggdrasil/protocol ../yggdrasil/protocol/yggdrasil.proto --python_out=rhc_worker_playbook/protocol --grpc_python_out=rhc_worker_playbook/protocol
