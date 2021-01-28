CONTRIB_DIR=$(pwd)/rhc_ansible_worker/contrib

mkdir $CONTRIB_DIR
python3 -m pip install --target $CONTRIB_DIR grpcio grpcio-tools ansible-runner
