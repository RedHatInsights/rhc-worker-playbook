# import os
# import sys
# from setuptools import setup, find_packages

import setuptools

setuptools.setup(
    name="yggdrasil-ansible-worker",
    version="0.1.0",
    author="Jeremy Crafts",
    author_email="jcrafts@redhat.com",
    description="Python worker for Yggdrasil",
    long_description="Listens on gRPC messages and launches Ansible with received playbooks",
    url="https://github.com/RedHatInsights/py-yggdrasil-grpc",
    packages=setuptools.find_packages(),
    classifiers=[
        "Programming Language :: Python :: 3",
        # "License :: OSI Approved :: MIT License",
        # "Operating System :: OS Independent",
    ],
    python_requires='>=3.6',


)