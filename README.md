# rhc-worker-playbook

`rhc-worker-playbook` is a worker for
[`yggdrasil`](https://github.com/RedHatInsights/yggdrasil) that receives
playbooks dispatched through `yggd`, executes them on the localhost using
`ansible-runner`, and returns the output through the console.redhat.com
[ingress](https://console.redhat.com/docs/api/ingress) service.

# Reporting Bugs

`rhc-worker-playbook` is included as part of Red Hat Enterprise Linux 8 and
newer. Please report any issues with rhc-worker-playbook against the
`rhc-worker-playbook` component of the appropriate Red Hat Enterprise Linux
product.

* [Link to new bug report for Red Hat Enterprise Linux 8](https://bugzilla.redhat.com/enter_bug.cgi?product=Red%20Hat%20Enterprise%20Linux%208&component=rhc-worker-playbook)
* [Link to new bug report for Red Hat Enterprise Linux 9](https://bugzilla.redhat.com/enter_bug.cgi?product=Red%20Hat%20Enterprise%20Linux%209&component=rhc-worker-playbook)

# Testing and Development

## Running tests

Any proposed code change in `rhc-worker-playbook` is automatically rejected by
`GitLab CI Pipelines` if the change causes test failures.

It is recommended for developers to run the test suite before submitting patch
for review. This allows to catch errors as early as possible.

### Preferred way to run the tests

The preferred way to run the linter tests is using `tox`. It executes tests in
an isolated environment, by creating separate `virtualenv` and installing
dependencies from the `test-requirements.txt` file, so the only package you
install is `tox` itself:

``` shell
$ python3 -m pip install tox
```

For more information, see [tox](https://tox.wiki/en/latest/). For example:

To run the default set of tests:

``` shell
$ tox
```

To run the style tests:

``` shell
$ tox -e linters
```
