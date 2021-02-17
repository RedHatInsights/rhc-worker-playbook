Name:       rhc-worker-playbook
Summary:    Red Hat connect worker for launching Ansible Runner
Version:    0.1.1
Release:    0%{?dist}
License:    GPLv2+
Source:     rhc-worker-playbook-0.1.1.tar.gz

%{?__python3:Requires: %{__python3}}
BuildRequires: python3-devel
BuildRequires: platform-python-pip

%description
Python-based worker for Red Hat connect, used to launch Ansible playbooks via Ansible Runner.

%prep
%setup -q

%install
%{make_install} BUILDROOT=%{buildroot} PREFIX=%{_prefix} LIBDIR=%{_libdir} LIBEXECDIR=%{_libexecdir} PYTHON=%{__python3}

%post

%preun

%postun

%clean
rm -rf %{buildroot}

%files
%{_libexecdir}/rhc/rhc-worker-playbook.worker
%{python3_sitelib}/rhc_worker_playbook/
%{python3_sitelib}/rhc_worker_playbook*.egg-info/
%{_libdir}/rhc-worker-playbook/

%doc

%changelog

