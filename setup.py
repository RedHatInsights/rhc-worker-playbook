import setuptools

setuptools.setup(
    name="rhc-ansible-worker",
    version="0.1.0",
    author="Jeremy Crafts",
    author_email="jcrafts@redhat.com",
    description="Python worker for RHC",
    long_description="Listens on gRPC messages and launches Ansible with received playbooks",
    url="https://github.com/RedHatInsights/rhc-ansible-worker",
    packages=setuptools.find_packages(),
    python_requires='>=3.6',
    entry_points={'console_scripts': ['ansible.worker = rhc_ansible_worker.server.serve']},
)
