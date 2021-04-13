PKGNAME=rhc-worker-playbook
TOPDIR=$(shell bash -c "pwd -P")
DISTDIR=$(TOPDIR)/dist
TARBALL=$(DISTDIR)/$(PKGNAME)-*.tar.gz

PREFIX		?= /usr/local
LIBDIR		?= $(PREFIX)/lib
LIBEXECDIR	?= $(PREFIX)/libexec
SYSCONFDIR	?= $(PREFIX)/etc
PYTHON		?= python3
CONFIG_DIR	?= $(SYSCONFDIR)/rhc/workers
CONFIG_FILE	?= $(CONFIG_DIR)/rhc-worker-playbook.toml
PYTHON_PKGDIR ?= $(LIBDIR)/python3.6/site-packages

.PHONY: tarball
tarball: $(TARBALL)
$(TARBALL): dev-lib-dir
	$(PYTHON) setup.py sdist

.PHONY: installed-lib-dir
installed-lib-dir:
	sed -i "/WORKER_LIB_DIR = .*/c\WORKER_LIB_DIR = \"$(LIBDIR)/$(PKGNAME)\"" ./rhc_worker_playbook/constants.py
	sed -i "/CONFIG_FILE = .*/c\CONFIG_FILE = \"$(CONFIG_FILE)\"" ./rhc_worker_playbook/constants.py
	sed -i "/sys.path.insert.*/c\sys.path.insert(1, \"$(PYTHON_PKGDIR)\")" ./scripts/rhc-worker-playbook.worker

.PHONY: dev-lib-dir
dev-lib-dir:
	sed -i "/WORKER_LIB_DIR = .*/c\WORKER_LIB_DIR = os.path.join(os.path.dirname(__file__), \"contrib\")" ./rhc_worker_playbook/constants.py
	sed -i "/CONFIG_FILE = .*/c\CONFIG_FILE = os.path.join(os.path.dirname(os.path.dirname(__file__)), \"rhc-worker-playbook.toml\")" ./rhc_worker_playbook/constants.py
	sed -i "/sys.path.insert.*/c\sys.path.insert(1, \"$(PYTHON_PKGDIR)\")" ./scripts/rhc-worker-playbook.worker

.PHONY: build
build:
	$(PYTHON) setup.py build
	$(PYTHON) -m pip wheel --wheel-dir=vendor --no-index --find-links vendor vendor/*.tar.gz

.PHONY: install
install: installed-lib-dir
	$(PYTHON) setup.py install --root=$(DESTDIR) --prefix=$(PREFIX) --install-scripts=$(LIBEXECDIR)/rhc --single-version-externally-managed --record /dev/null
	$(PYTHON) -m pip install --target $(DESTDIR)$(LIBDIR)/$(PKGNAME) --no-index --find-links vendor vendor/*.whl
	[[ -e $(DESTDIR)$(CONFIG_FILE) ]] || install -D -m644 ./rhc-worker-playbook.toml $(DESTDIR)$(CONFIG_FILE)

.PHONY: uninstall
uninstall:
	rm -rf $(LIBEXECDIR)/rhc/$(PKGNAME).worker
	rm -rf $(LIBDIR)/python*/site-packages/$(PKGNAME)*
	rm -rf $(LIBDIR)/$(PKGNAME)
.PHONY: clean

