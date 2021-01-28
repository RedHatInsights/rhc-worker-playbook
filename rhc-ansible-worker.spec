%define worker_lib_dir %{_libdir}/rhc-ansible-worker

Name:       rhc-ansible-worker
Summary:    Red Hat connect worker for launching Ansible Runner
Version:    0.1.0
Release:    0%{?dist}
License:    GPLv2+
Source:     rhc-ansible-worker-0.1.0.tar.gz

%{?__python3:Requires: %{__python3}}
Requires:      ansible
Requires:      python3-PyYAML
BuildRequires: python3-devel

%description
Python-based worker for Red Hat connect, used to launch Ansible playbooks via Ansible Runner.

%prep
%setup -q
sed -i "/WORKER_LIB_DIR = .*/c\WORKER_LIB_DIR = \"%{worker_lib_dir}\"" rhc_ansible_worker/constants.py

%install
%{__python3} setup.py install --install-scripts %{_libexecdir}/rhc --root %{buildroot}
%{__python3} -m pip install --target %{buildroot}%{_libdir}/rhc-ansible-worker ansible-runner

%post

%preun

%postun

%clean
rm -rf %{buildroot}

%files
%{_libexecdir}/rhc/ansible.worker
%{python3_sitelib}/rhc_ansible_worker/
%{python3_sitelib}/rhc_ansible_worker*.egg-info/
%{_libdir}/rhc-ansible-worker/

%doc

%changelog

