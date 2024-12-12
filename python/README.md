To update package versions in this directory:

* Change the version string pinned in `requirements.in`
* Download the wheel or tarball of the new version from pypi.org
* Update the entry in SHA256SUM with the correct hash
* Run the following command to regenerate `requirements.txt`

```
python -m piptools compile --allow-unsafe --find-links=. --generate-hashes --no-index requirements.in > requirements.txt
```
