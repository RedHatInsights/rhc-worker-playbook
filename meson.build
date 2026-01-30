project('rhc-worker-playbook', version : '0.2.6')

go = find_program('go')
dbus = dependency('dbus-1', version: '>=1.12')
systemd = dependency('systemd', version: '>=239')
yggdrasil = dependency('yggdrasil', version: '>=0.4.2')

if get_option('vendor')
  meson.add_dist_script(join_paths('build-aux', 'vendor.sh'))
endif

goldflags = get_option('goldflags')
goldflags += ' -X "github.com/redhatinsights/rhc-worker-playbook/internal/constants.Version=' + meson.project_version() + '"'
goldflags += ' -X "github.com/redhatinsights/rhc-worker-playbook/internal/constants.PrefixDir=' + get_option('prefix') + '"'
goldflags += ' -X "github.com/redhatinsights/rhc-worker-playbook/internal/constants.SysconfDir=' + get_option('sysconfdir') + '"'
goldflags += ' -X "github.com/redhatinsights/rhc-worker-playbook/internal/constants.LocalStateDir=' + get_option('localstatedir') + '"'
goldflags += ' -X "github.com/redhatinsights/rhc-worker-playbook/internal/constants.LibDir=' + get_option('prefix') / get_option('libdir') + '"'
goldflags += ' -X "github.com/redhatinsights/rhc-worker-playbook/internal/constants.DataDir=' + get_option('prefix') / get_option('datadir') + '"'

gobuildflags = get_option('gobuildflags')

config_data = configuration_data(
  {
    'prefix': get_option('prefix'),
    'bindir': join_paths(get_option('prefix'), get_option('bindir')),
    'sbindir': join_paths(get_option('prefix'), get_option('sbindir')),
    'configdir': join_paths(get_option('sysconfdir'), meson.project_name()),
    'libexecdir': join_paths(get_option('prefix'), get_option('libexecdir')),
    'localstatedir': join_paths(get_option('prefix'), get_option('localstatedir')),
    'sharedstatedir': join_paths(get_option('prefix'), get_option('sharedstatedir')),
    'datadir': join_paths(get_option('prefix'), get_option('datadir')),
    'worker_user': yggdrasil.get_variable(pkgconfig: 'worker_user'),
  },
)

subdir('ansible')
subdir('data')
subdir('dist')
subdir('python')

custom_target(
  command: [
    go,
    'build',
    gobuildflags,
    '-o', '@OUTPUT@',
    '-ldflags', goldflags,
    'github.com/redhatinsights/rhc-worker-playbook',
  ],
  output: 'rhc-worker-playbook',
  build_by_default: true,
  install: true,
  install_dir: get_option('libexecdir'),
)
