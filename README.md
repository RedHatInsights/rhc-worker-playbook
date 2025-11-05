# rhc-worker-playbook

`rhc-worker-playbook` is a worker for [`yggdrasil`](https://github.com/RedHatInsights/yggdrasil)
that receives playbooks dispatched through `yggd`, executes them on the localhost using
`ansible-runner`, and returns the output through the console.redhat.com
[ingress](https://console.redhat.com/docs/api/ingress) service.

## Reporting Bugs

`rhc-worker-playbook` is included as part of Red Hat Enterprise Linux 8 and newer. Please report any
issues with rhc-worker-playbook to [Red Hat Jira](https://issues.redhat.com), with project set to
RHINENG and component set to "Remediations Packages".

## How to contribute

To develop and contribute, follow the
[CONTRIBUTING](https://github.com/RedHatInsights/rhc-worker-playbook/blob/main/CONTRIBUTING.md)
guide.
