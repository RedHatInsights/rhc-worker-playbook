# How to build a development environment

`rhc-worker-playbook` is part of the `yggd` environment, so it will need the
same as
[`yggd`](https://github.com/RedHatInsights/yggdrasil/blob/main/CONTRIBUTING.md)
to be tested and developed.

# Prerequisites

Being a yggdrasil worker, `rhc-worker-playbook` requires a similar development
environment to
[`yggdrasil`](https://github.com/RedHatInsights/yggdrasil/blob/main/CONTRIBUTING.md).
It is recommended that you first ensure you have a working yggdrasil development
set up, although it is possible to develop `rhc-worker-playbook` using a
packaged version of yggdrasil.

`rhc-worker-playbook` executes playbooks that assume they are running as a
privileged user. As a result, the included systemd service runs as `root`.

## `yggd`

Make sure `yggd` is running and listening on the system bus. If using a
`yggdrasil` package, starting the `yggdrasil.service` unit should be sufficient.
Be sure to configure `/etc/yggdrasil/config.toml` to connect to an MQTT broker,
should you wish to send messages over the network.

## `mqttcli`

It is recommended to install a simple MQTT publish/subscribe utility. Included
in Fedora is `mqttcli`, a pair of simple client utilities for such purpose.

## HTTP server

An HTTP server is used to request payloads from localhost. This does not need to
be more complicated than using the Python's `SimpleHTTPServer` module.

# Compilation

It is possible, though awkward, to run `rhc-worker-playbook` directly from the
project repository. If you're working on a Fedora-derived distribution, use the
included `srpm` meson target to create a SRPM. This SRPM can then be built using
`mock` or `koji` or any other RPM build system.

```console
meson setup -Dbuild_srpm=True builddir
meson compile srpm -C builddir
mock --rebuild ./builddir/dist/srpm/*.src.rpm
```

# Test environment Quickstart

This environment recreates the `rhc-worker-playbook`, in which an `ansible
playbook` is sent through the MQTT broker, `yggdrasil` processes it and send it
to the worker to execute it.


## TERMINAL 1

 Start the `mosquitto` service. Then run an *HTTP server* that will serve the
 payload.

```console
$ systemctl start mosquitto
$ python3 -m http.server --directory ./testdata 8000
```

## TERMINAL 2

Configure yggdrasil to connect to the local `moquitto` broker and increase the
log level.

```console
cat /etc/yggdrasil/config.toml

protocol = "mqtt"
server = ["tcp://localhost:1883"]
log-level = "debug"
```

Start the service:

```console
sudo systemctl start yggdrasil.service
```

## TERMINAL 3

Assuming you built and installed the `rhc-worker-package` as described in
[#Compilation](#compilation), start
`com.redhat.Yggdrasil1.Worker1.rhc_worker_playbook.service`:

```console
sudo systemctl start com.redhat.Yggdrasil1.Worker1.rhc_worker_playbook.service
```

## TERMINAL 4

Generate a data message and publish it. `yggctl` (installed as part of the
`yggdrasil` package) can easily generate data messages in the JSON format
expected by `yggd`.

```console
echo '"http://localhost:8000/insights_remove.yml"' | \
  yggctl generate data-message --directive rhc_worker_playbook -
```

This can then be piped to `pub` to publish the message:

```console 
echo '"http://localhost:8000/insights_remove.yml"' | \
  yggctl generate data-message --directive rhc_worker_playbook - | \
  pub -broker tcp://localhost:1883 -topic yggdrasil/$(cat /var/lib/yggdrasil/client-id)/data/in
```

 The client ID can be found in the output of [TERMINAL 2](#terminal-2), when
 `yggdrasil` is launched. The topics `yggd` subscribes to should be printed in
 the console output.

 ```console
 journalctl -b -u yggdrasil.service | grep -r "yggdrasil/.*/data/in"
 ```

# How to contribute

 ## Running tests

Any proposed code change in `rhc-worker-playbook` is automatically rejected by
"GitHub Actions" if the change causes test failures.

It is recommended for developers to run the test suite before submitting patch
for review. This allows to catch errors as early as possible.

### Preferred way to run the tests

``` shell
go test ./
```

## Code Guidelines
- Commits follow the [Conventional
  Commits](https://www.conventionalcommits.org/) pattern.
- Commit messages should include a concise subject line that completes the
  following phrase: "when applied, this commit will...". The body of the commit
  should further expand on this statement with additional relevant details.
- Files should be formatted using `gofmt` before committing changes.
- A release branch, `release-0.1` exists for maintaining the 0.1.x branch. This
  branch is intended to maintain RHEL8 and RHEL9 compatibility.
