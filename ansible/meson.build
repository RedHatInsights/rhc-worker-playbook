find_program('jq')
find_program('ansible-galaxy')

collections = files('ansible-posix-2.2.1.tar.gz', 'community-general-12.6.2.tar.gz')

meson.add_install_script('install.sh', collections)
