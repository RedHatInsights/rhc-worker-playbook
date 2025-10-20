import setuptools

setuptools.setup(
    name="rhc_worker_playbook",
    version="0.1.11",
    author="Jeremy Crafts",
    author_email="jcrafts@redhat.com",
    description="Python worker for RHC",
    long_description="Listens on gRPC messages and launches Ansible with received playbooks",
    url="https://github.com/RedHatInsights/rhc-ansible-worker",
    packages=setuptools.find_packages(),
    python_requires='>=3.6,<3.13',
    scripts=[
        "scripts/rhc-worker-playbook.worker"
    ],
    zip_safe=False,
)
