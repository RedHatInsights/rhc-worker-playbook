PYTHON		?= python3.9

PKGNAME=rhc-worker-playbook
PKGVER = $(shell $(PYTHON) setup.py --version | tr -d '\n')
_SHORT_COMMIT = $(shell git rev-parse --short HEAD | tr -d '\n')
_LATEST_TAG = $(shell git describe --tags --abbrev=0 --always | tr -d '\n')
_NUM_COMMITS_SINCE_LATEST_TAG = $(shell git rev-list $(_LATEST_TAG)..HEAD --count | tr -d '\n')
RELEASE = $(shell printf "99.%s.git.%s" $(_NUM_COMMITS_SINCE_LATEST_TAG) $(_SHORT_COMMIT))

PREFIX		?= /usr/local
LIBDIR		?= $(PREFIX)/lib
LIBEXECDIR	?= $(PREFIX)/libexec
SYSCONFDIR	?= $(PREFIX)/etc
CONFIG_DIR	?= $(SYSCONFDIR)/rhc/workers
CONFIG_FILE	?= $(CONFIG_DIR)/rhc-worker-playbook.toml
WORKER_LIB_DIR ?= $(LIBDIR)/$(PKGNAME)
PYTHON_PKGDIR ?= $(shell /usr/libexec/platform-python -Ic "from distutils.sysconfig import get_python_lib; print(get_python_lib())")

.PHONY: build
build: rhc_worker_playbook/constants.py scripts/rhc-worker-playbook.worker
	$(PYTHON) setup.py build
	$(PYTHON) -m pip wheel --wheel-dir=vendor --no-index --find-links vendor vendor/*.tar.gz

rhc_worker_playbook/constants.py: rhc_worker_playbook/constants.py.in
	sed \
		-e 's,[@]CONFIG_FILE[@],$(CONFIG_FILE),g' \
		-e 's,[@]WORKER_LIB_DIR[@],$(WORKER_LIB_DIR),g' \
		$^ > $@

scripts/rhc-worker-playbook.worker: scripts/rhc-worker-playbook.worker.in
	sed \
		-e 's,[@]PYTHON_PKGDIR[@],$(PYTHON_PKGDIR),g' \
		$^ > $@

.PHONY: install
install: 
	$(PYTHON) setup.py install --root=$(DESTDIR) --prefix=$(PREFIX) --install-scripts=$(LIBEXECDIR)/rhc --single-version-externally-managed --record /dev/null
	$(PYTHON) -m pip install --target $(DESTDIR)$(LIBDIR)/$(PKGNAME) --no-index --find-links vendor vendor/*.whl
	[[ -e $(DESTDIR)$(CONFIG_FILE) ]] || install -D -m644 ./rhc-worker-playbook.toml $(DESTDIR)$(CONFIG_FILE)

.PHONY: uninstall
uninstall:
	rm -rf $(LIBEXECDIR)/rhc/$(PKGNAME).worker
	rm -rf $(LIBDIR)/python*/site-packages/$(PKGNAME)*
	rm -rf $(LIBDIR)/$(PKGNAME)

.PHONY: clean
clean:
	rm rhc_worker_playbook/constants.py
	rm scripts/rhc-worker-playbook.worker
	rm rhc-worker-playbook.spec

.PHONY: tarball
tarball:
	$(PYTHON) setup.py sdist
# 	what the heck are we doing here
# 	rm -f dist/rhc_worker_playbook-$(PKGVER).tar.gz
# 	mv rhc_worker_playbook-$(PKGVER) $(PKGNAME)-$(PKGVER)
# 	tar czf $(PKGNAME)-$(PKGVER).tar.gz $(PKGNAME)-$(PKGVER)
# 	mv $(PKGNAME)-$(PKGVER).tar.gz dist/$(PKGNAME)-$(PKGVER).tar.gz
# 	rm -rf $(PKGNAME)-$(PKGVER)

.PHONY: rhc-worker-playbook.spec
rhc-worker-playbook.spec: rhc-worker-playbook.spec.in
	sed \
		-e 's,[@]PKGVER[@],$(PKGVER),g' \
		-e 's,[@]RELEASE[@],$(RELEASE),g' \
		$< > $@.tmp && mv $@.tmp $@
