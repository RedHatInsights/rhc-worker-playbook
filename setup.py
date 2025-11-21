import setuptools

setuptools.setup(
    name="rhc-worker-playbook",
    version="0.1.11",
    author="Jeremy Crafts",
    author_email="jcrafts@redhat.com",
    description="Python worker for RHC",
    long_description="Listens on gRPC messages and launches Ansible with received playbooks",
    url="https://github.com/RedHatInsights/rhc-ansible-worker",
    packages=setuptools.find_packages(),
    python_requires=">=3.9",
    entry_points={
        "console_scripts": [
            "rhc-worker-playbook.worker = rhc_worker_playbook.server:serve"
        ]
    },
    setup_requires=["wheel"],
    install_requires=[
        "ansible-runner==2.1.1",
        "grpcio<1.56",  # required by protobuf<=3.20
        "grpcio-tools<1.49",  # required by protobuf<=3.20
        "jsonschema",
        "protobuf<=3.20",  # required by rhc_worker_playbook/protocol/*
        "requests",
        "setuptools<81",  # deps need pkg_resources at runtime
        "toml",
    ],
    zip_safe=False,
)
