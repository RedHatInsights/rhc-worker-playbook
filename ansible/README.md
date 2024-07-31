This subdirectory contains the build and installation instructions for the
Ansible collections bundled as part of rhc-worker-playbook. These collections
are installed to a custom path rather than into the system-wide collections
path. In order to ensure offline builds work correctly, the collections have
been downloaded as offline archives.

To update a collection to a newer version, run the following:

```console
ansible-galaxy collection download -p . ansible.posix community.general
```
