# How to build development environment

`rhc-worker-playbook` is part of the `yggd` environment, so it will need the same as [`yggd`](https://github.com/RedHatInsights/yggdrasil/blob/main/CONTRIBUTING.md) to be tested and developed.

# Pre-requisites
*yggdrasil*: Connects the MQTT broker with the appropriate worker, `rhc-worker-playbook` in this case.

It can be compiled using the including `Makefile` or executed just with the `go run` command. It requires `pub` and `sub` modules. To get these modules:

```console
$ go get git.sr.ht/~spc/mqttcli
$ go install git.sr.ht/~spc/mqttcli/cmd/...
```

*MQTT broker*: Needed to publish messages. [Mosquitto](https://mosquitto.org/) is easy to set up and an excellent offline local broker. The online [solution](https://test.mosquitto.org/) can be used but is not recommended.


*HTTP server*: An optional HTTP server used to request payloads from localhost. This does not need to be more complicated than Python's `SimpleHTTPServer` module.

# Install rhc-worker-playbook develop setup

`rhc-worker-playbook` need some devel packages prior to install the `develop-setup` script.

```console
$ sudo dnf install c-ares-devel openssl-devel python3-devel gcc gcc-c++
$ ./develop-setup.sh
```

# Test environment Quickstart

This environment recreates the `rhc-worker-playbook`, in which an `ansible playbook` is sent through the MQTT broker, `yggdrasil` process it and send it to the worker to execute it.


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
```

**TERMINAL 3**

Associate the `@yggd` socket with the `rhc-worker-playbook` worker.

```console
sudo YGG_SOCKET_ADDR=unix:@yggd python3 /usr/libexec/rhc/rhc-worker-playbook.worker
```

**TERMINAL 4**

Execute the publish message in the "control/in" topic. Using `yggctl` to generate the data message with the ansible playbook `test-module.yml` served by the HTTP server and `rhc-worker-playbook` as the directive.

```console
 go run ./cmd/yggctl generate data-message --directive "rhc-worker-playbook" \"http://localhost:8000/test-module.yml\" | pub -broker tcp://localhost:1883 -topic yggdrasil/$CLIENT_ID/data/in
 ```

# How to contribute

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
## Code Guidelines
- Commits follow the [Conventional Commits](https://www.conventionalcommits.org/) pattern.
- Commit messages should include a concise subject line that completes the following phrase: "when applied, this commit will...". The body of the commit should further expand on this statement with additional relevant details.
