subdir('dbus')
subdir('systemd')

install_data(
  'rhc-worker-playbook.toml',
  install_dir: get_option('sysconfdir') / meson.project_name(),
)

install_data(
  'ansibleRunnerJobEvent.yaml',
  install_dir: get_option('libdir') / meson.project_name(),
)
