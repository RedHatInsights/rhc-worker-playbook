PKGNAME=rhc-worker-playbook
TOPDIR=$(shell bash -c "pwd -P")
DISTDIR=$(TOPDIR)/dist
TARBALL=$(DISTDIR)/$(PKGNAME)-*.tar.gz

PREFIX        ?= /usr/local
LIBDIR        ?= $(PREFIX)/lib
LIBEXECDIR    ?= $(PREFIX)/libexec
PYTHON	      ?= python3

.PHONY: tarball
tarball: $(TARBALL)
$(TARBALL): dev-lib-dir
	$(PYTHON) setup.py sdist

.PHONY: installed-lib-dir
installed-lib-dir:
	sed -i "/WORKER_LIB_DIR = .*/c\WORKER_LIB_DIR = \"$(LIBDIR)/$(PKGNAME)\"" ./rhc_worker_playbook/constants.py

.PHONY: dev-lib-dir
dev-lib-dir:
	sed -i "/WORKER_LIB_DIR = .*/c\WORKER_LIB_DIR = os.path.join(os.path.dirname(__file__), 'contrib')" ./rhc_worker_playbook/constants.py

.PHONY: build
build:
	$(PYTHON) setup.py build

.PHONY: install
install: installed-lib-dir
	$(PYTHON) setup.py install --root=$(DESTDIR) --prefix=$(PREFIX) --install-scripts=$(LIBEXECDIR)/rhc --single-version-externally-managed --record /dev/null
	$(PYTHON) -m pip install --target $(DESTDIR)$(LIBDIR)/$(PKGNAME) --no-index --find-links vendor vendor/*

.PHONY: uninstall
uninstall:
	rm -rf $(LIBEXECDIR)/rhc/$(PKGNAME).worker
	rm -rf $(LIBDIR)/python*/site-packages/$(PKGNAME)*
	rm -rf $(LIBDIR)/$(PKGNAME)
.PHONY: clean

