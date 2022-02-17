CONTRIB_DIR=$(pwd -P)/rhc_worker_playbook/contrib

mkdir $CONTRIB_DIR
python3 -m pip install --target $CONTRIB_DIR grpcio==1.38.1 grpcio-tools==1.38.1 ansible-runner==2.1.1
