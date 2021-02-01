%define _prefix /usr/local
%define worker_lib_dir %{_libdir}/rhc-worker-playbook

Name:       rhc-worker-playbook
Summary:    Red Hat connect worker for launching Ansible Runner
Version:    0.1.0
Release:    0%{?dist}
License:    GPLv2+
Source:     rhc-worker-playbook-0.1.0.tar.gz

%{?__python3:Requires: %{__python3}}
Requires:      python3-PyYAML
BuildRequires: python3-devel

%description
Python-based worker for Red Hat connect, used to launch Ansible playbooks via Ansible Runner.

%prep
%setup -q
sed -i "/WORKER_LIB_DIR = .*/c\WORKER_LIB_DIR = \"%{worker_lib_dir}\"" rhc_worker_playbook/constants.py

%install
%{__python3} setup.py install --install-scripts %{_libexecdir}/redhat-connect --root %{buildroot}
%{__python3} -m pip install --target %{buildroot}%{_libdir}/rhc-worker-playbook ansible-runner grpcio grpcio-tools

%post

%preun

%postun

%clean
rm -rf %{buildroot}

%files
%{_libexecdir}/redhat-connect/rhc-worker-playbook.worker
%{python3_sitelib}/rhc_worker_playbook/
%{python3_sitelib}/rhc_worker_playbook*.egg-info/
%{_libdir}/rhc-worker-playbook/

%doc

%changelog

