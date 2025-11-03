PYTHON		?= python3.9

PKGNAME=rhc-worker-playbook
PKGVER = $(shell $(PYTHON) setup.py --version | tr -d '\n')
_SHORT_COMMIT = $(shell git rev-parse --short HEAD | tr -d '\n')
_LATEST_TAG = $(shell git describe --tags --abbrev=0 --always | tr -d '\n')
_NUM_COMMITS_SINCE_LATEST_TAG = $(shell git rev-list $(_LATEST_TAG)..HEAD --count | tr -d '\n')
RELEASE = $(shell printf "99.%s.git.%s" $(_NUM_COMMITS_SINCE_LATEST_TAG) $(_SHORT_COMMIT))

PREFIX		?= /usr
LIBDIR		?= $(PREFIX)/lib
LIBEXECDIR	?= $(PREFIX)/libexec
WORKER_FILE	?= $(LIBEXECDIR)/rhc/$(PKGNAME).worker
CONFIG_FILE	?= /etc/rhc/workers/$(PKGNAME).toml

SOURCES := $(shell git ls-files '*.py' rhc-worker-playbook.toml)

dist: rhc_worker_playbook/constants.py $(SOURCES)
	$(PYTHON) setup.py sdist
	touch $@  # ensure target is newer than prerequisites

wheels: rhc_worker_playbook/constants.py $(SOURCES)
	$(PYTHON) -m pip wheel --wheel-dir=wheels .
	touch $@  # ensure target is newer than prerequisites

rhc_worker_playbook/constants.py: rhc_worker_playbook/constants.py.in
	sed \
		-e 's,[@]CONFIG_FILE[@],$(CONFIG_FILE),g' \
		-e 's,[@]WORKER_LIB_DIR[@],$(LIBDIR)/$(PKGNAME),g' \
		$^ > $@

.PHONY: install
install: wheels
	# vendored deps
	$(PYTHON) -m pip install wheels/ansible* wheels/grpcio* wheels/protobuf* wheels/requests* wheels/toml* --no-index --find-links wheels --no-compile --target $(DESTDIR)$(LIBDIR)/$(PKGNAME)
	#
	# rhc-worker-playbook package
	$(PYTHON) -m pip install wheels/rhc_worker_playbook* --no-index --no-deps --no-compile --prefix=$(DESTDIR)$(PREFIX)
	#
	# rhc-worker-playbook.worker
	install -Dm 755 $(DESTDIR)$(PREFIX)/bin/rhc-worker-playbook.worker $(DESTDIR)$(WORKER_FILE)
	rm -f $(DESTDIR)$(PREFIX)/bin/rhc-worker-playbook.worker
	#
	# rhc-worker-playbook.toml
	install -Dm 644 --backup ./rhc-worker-playbook.toml $(DESTDIR)$(CONFIG_FILE)

.PHONY: uninstall
uninstall:
	# vendored deps
	rm -r $(DESTDIR)$(LIBDIR)/$(PKGNAME)
	# rhc-worker-playbook package
	rm -r $(DESTDIR)$(LIBDIR)/$(PYTHON)/site-packages/rhc_worker_playbook*
	# rhc-worker-playbook.worker
	rm $(DESTDIR)$(WORKER_FILE)
	# rhc-worker-playbook.toml
	rm $(DESTDIR)$(CONFIG_FILE)

.PHONY: clean
clean:
	rm -rf dist
	rm -rf wheels
	rm -f rhc_worker_playbook/constants.py
	rm -f rhc-worker-playbook.spec

rhc-worker-playbook.spec: rhc-worker-playbook.spec.in
	sed \
		-e 's,[@]PKGVER[@],$(PKGVER),g' \
		-e 's,[@]RELEASE[@],$(RELEASE),g' \
		$< > $@.tmp && mv $@.tmp $@
