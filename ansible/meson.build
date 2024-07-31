find_program('jq')
find_program('ansible-galaxy')

collections = files('ansible-posix-1.5.4.tar.gz', 'community-general-9.2.0.tar.gz')

meson.add_install_script('install.sh', collections)
