# How to build a development environment

`rhc-worker-playbook` is part of the `yggd` environment, so it will need the same as [`yggd`](https://github.com/RedHatInsights/yggdrasil/blob/main/CONTRIBUTING.md) to be tested and developed.

# Pre-requisites
*yggdrasil/rhc*: Connects the MQTT broker with the appropriate worker, `rhc-worker-playbook` in this case.

It can be compiled using the including `Makefile` or executed just with the `go run` command. `rhc`is part of RHEL systems, it can be used instead of using `yggdrasil`.
This requires `pub` and `sub` modules. To get these modules:

```console
$ go get git.sr.ht/~spc/mqttcli
$ go install git.sr.ht/~spc/mqttcli/cmd/...
```

*MQTT broker*: Needed to publish messages. [Mosquitto](https://mosquitto.org/) is easy to set up and an excellent broker that can be run offline and locally. The online [solution](https://test.mosquitto.org/) can be used but is not recommended.


*HTTP server*: An optional HTTP server used to request payloads from localhost. This does not need to be more complicated than using the Python's `SimpleHTTPServer` module.

# Installation

To install for development:

```bash
# RHEL 8 package names; change "39" to "3" on RHEL 9
dnf install python39 python39-devel python39-setuptools make ansible-core
CONFIG_DIR=$(realpath .) make rhc_worker_playbook/constants.py

python3.9 -m venv .venv
source .venv/bin/activate
pip install --editable .

# Connect to rhcd.service
ss | grep yggd-dispatcher
YGG_SOCKET_ADDR=unix:@yggd-dispatcher-... $(which rhc-worker-playbook.worker)
```

To install system-wide (**dangerous!**):

```bash
# RHEL 8 package names; change "39" to "3" on RHEL 9
dnf install python39 python39-devel python39-setuptools make ansible-core
make wheels

# preview install tree
DESTDIR=fakeroot make install

# install; rhcd.service automatically starts worker
sudo make install
```

# Test environment Quickstart

This environment recreates the `rhc-worker-playbook`, in which an `ansible playbook` is sent through the MQTT broker, `yggdrasil` processes it and send it to the worker to execute it.


**TERMINAL 1**

Run an *HTTP server* that will serve the payload. Then run `mosquitto` service.

```console
$ nohup python3 -m http.server 8000 > /dev/null 2>&1 &
$ mosquitto
```

**TERMINAL 2**

Run `yggdrasil` specifying `mosquitto` as the MQTT server.

```console
$ sudo go run ./cmd/yggd --server tcp://localhost:1883 --log-level trace --socket-addr @yggd
...
subscribed to topic: yggdrasil/localhost-22278168-85a6-11ec-a65c-fa163e3b5a61/data/in
...
```
Or, if it is using via `rhc`.

Configure in rhc the `moquitto` connection and the log level.

```console
cat /etc/rhc/config.toml
# rhc global configuration settings

broker = ["tcp://localhost:1883"]
cert-file = "/etc/pki/consumer/cert.pem"
key-file = "/etc/pki/consumer/key.pem"
log-level = "bug"
```

**TERMINAL 3**

Associate the `@yggd` socket with the `rhc-worker-playbook` worker.

```console
sudo YGG_SOCKET_ADDR=unix:@yggd python3 /usr/libexec/rhc/rhc-worker-playbook.worker
```
This step is not needed if using the worker through `rhc`.

**TERMINAL 4**

Execute the publish message in the "control/in" topic. Using `yggctl` to generate the data message with the Ansible playbook `test-module.yml` served by the HTTP server and `rhc-worker-playbook` as the directive.

```console
 go run ./cmd/yggctl generate data-message --directive "rhc-worker-playbook" \"http://localhost:8000/test-module.yml\" | pub -broker tcp://localhost:1883 -topic yggdrasil/$CLIENT_ID/data/in
 ```


 `$CLIENT_ID` can be found in the prompr of the TERMINAL 2, when `yggdrasil` is launched. The output tells the topic it is subscribed, in there the `$CLIENT_ID` can be found. Or in the logs of the `rhc`.

 ```console
subscribed to topic: yggdrasil/localhost-22278168-85a6-11ec-a65c-fa163e3b5a61/data/in
CLIENT_ID = localhost-22278168-85a6-11ec-a65c-fa163e3b5a61
 ```

Alternatively, `yggdrasil` has a variable in its config file to determine the `client_id`. If the worker is running through `rhc` this `$CLIENT_ID` can be found in the logs of `journalctl`.

```console
# journalctl -u rhcd -f

```

# How to contribute

To lint changes:

```bash
pip install pre-commit
pre-commit run
```

Additionally:

- Commit messages should include a concise subject line that completes the following phrase: "when
  applied, this commit will...". The body of the commit should further expand on this statement with
  additional relevant details.
- The `release-0.1` branch provides a Python implementation, for RHEL 8 and RHEL 9.
