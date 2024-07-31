python = import('python')
py_installation = python.find_installation()

sha256sum = find_program('sha256sum')
run_command(sha256sum, '--check', files('SHA256SUM'), check: true)

find_program('jq')

meson.add_install_script(
  'install.sh',
  py_installation.full_path(),
  '-m', 'pip', 'install',
  '--upgrade',
  '--no-index',
  '--requirement', files('requirements.txt'),
)
