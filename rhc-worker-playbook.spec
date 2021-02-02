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

%install
%{make_install} BUILDROOT=%{buildroot} PREFIX=%{_prefix} LIBDIR=%{_libdir} LIBEXECDIR=%{_libexecdir}

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

