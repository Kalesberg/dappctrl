#!/usr/bin/python
# -*- coding: utf-8 -*-

""" Initializer on pure Python 2.7 """
import logging
import sys
from time import time, sleep
from re import search
from time import sleep
from urllib import URLopener
from urllib2 import urlopen
from subprocess import Popen, PIPE, STDOUT
from platform import linux_distribution
from os.path import isfile, isdir
from os import remove, mkdir
from shutil import copyfile
from codecs import open
from json import load, dump

"""
Exit code:
    1 - Problem with get or upgrade systemd ver
    2 - If version of Ubuntu lower than 16
    3 - If sysctl net.ipv4.ip_forward = 0 after sysctl -w net.ipv4.ip_forward=1
    4 - Problem when call system command from subprocess
    5 - Problem with operation R/W unit file 
    6 - Problem with operation download file 
    7 - Problem with operation R/W server.conf 
    8 - Default DB conf is empty, and no section 'DB' in dappctrl-test.config.json
    9 - Check the run of the database is negative
    10 - Problem with read dapp cmd from file
"""

log_conf = dict(
    filename='initializer.log',
    datefmt='%m/%d %H:%M:%S',
    format='%(levelname)7s [%(lineno)3s] %(message)s')
log_conf.update(level='DEBUG')
logging.basicConfig(**log_conf)
logging.getLogger().addHandler(logging.StreamHandler())

main_conf = dict(
    iptables=dict(
        link_download='http://art.privatix.net/',
        file_download=[
            'vpn.tar.xz',
            'common.tar.xz',
            'systemd-nspawn@vpn.service',
            'systemd-nspawn@common.service'],
        path_download='/var/lib/container/',
        path_vpn='vpn/',
        path_com='common/',
        path_unit='/lib/systemd/system/',
        openvpn_conf='/etc/openvpn/config/server.conf',
        openvpn_fields=[
            'server {} {}',
            'push "route {} {}"'
        ],

        unit_vpn='systemd-nspawn@vpn.service',
        unit_com='systemd-nspawn@common.service',
        unit_field={
            'ExecStop=/sbin/sysctl': False,
            'ExecStopPost=/sbin/sysctl': False,

            'ExecStop=/sbin/iptables': 'ExecStop=/sbin/iptables -t nat -A POSTROUTING -s {} -o {} -j MASQUERADE\n',
            'ExecStartPre=/sbin/iptables': 'ExecStartPre=/sbin/iptables -t nat -A POSTROUTING -s {} -o {} -j MASQUERADE\n',
            'ExecStopPost=/sbin/iptables': 'ExecStopPost=/sbin/iptables -t nat -A POSTROUTING -s {} -o {} -j MASQUERADE\n',
        }

    ),
    build={
        'cmd': '/opt/privatix/initializer/dappinst -dappvpnconftpl=\'{}\' -dappvpnconf={} -connstr=\'{}\'',
        'cmd_path': '.dapp_cmd',
        'db_conf': {
            "dbname": "dappctrl",
            "sslmode": "disable",
            "user": "postgres",
            "host": "localhost",
            "port": "5432"
            },
        'db_log': '/var/lib/container/common/var/log/postgresql/postgresql-10-main.log',
        'db_stat': 'database system is ready to accept connections',
        'dappvpnconf_path': '/var/lib/container/vpn/opt/privatix/config/dappvpn.config.json',
        'conf_link': 'https://raw.githubusercontent.com/Privatix/dappctrl/develop/dappctrl.config.json',
        'templ': 'https://raw.githubusercontent.com/Privatix/dappctrl/develop/svc/dappvpn/dappvpn.config.json',
    },
    addr='10.217.3.0',
    mask=['/24', '255.255.255.0'],
    mark_final='/var/run/installer.pid',
)


class CMD:
    recursion = 0

    def _rolback(self, sysctl, code):

        # Rolback net.ipv4.ip_forward
        if not sysctl:
            logging.info('Rolback ip_forward')
            cmd = '/sbin/sysctl -w net.ipv4.ip_forward=0'
            self._sys_call(cmd)
        sys.exit(code)

    def _file_rw(self, p, w=False, data=None, log=None):
        try:
            if log:
                logging.info(log)
            if w:
                f = open(p, 'w')
                if data:
                    f.writelines(data)
                f.close()
            else:
                f = open(p, 'r')
                data = f.readlines()
                f.close()
                return data
        except BaseException as rwexpt:
            logging.error('R/W File: {}'.format(rwexpt))
            return False

    def _sys_call(self, cmd, sysctl=False):
        resp = Popen(cmd, shell=True, stdout=PIPE,
                     stderr=STDOUT).communicate()
        logging.debug('Sys call: {}'.format(resp))
        if resp[1]:
            logging.error(resp[1])
            self._rolback(sysctl, 4)
        return resp[0]

    def _upgr_deb_pack(self, v):
        logging.info('Debian: {}'.format(v))

        cmd = 'echo deb http://http.debian.net/debian jessie-backports main ' \
              '> /etc/apt/sources.list.d/jessie-backports.list'
        logging.info('Add jessie-backports.list')
        self._sys_call(cmd)
        self._sys_call(cmd='apt-get install lshw -y')

        logging.info('Update')
        self._sys_call('apt-get update')
        self.__upgr_sysd(
            cmd='apt-get -t jessie-backports install systemd -y')

        logging.debug('Install systemd-container')
        self._sys_call('apt-get install systemd-container -y')

    def _upgr_ub_pack(self, v):
        logging.info('Ubuntu: {}'.format(v))

        if int(v.split('.')[0]) < 16:
            logging.error('Your version of Ubuntu is lower than 16. '
                          'It is not supported by the program')
            sys.exit(2)
        self._sys_call('apt-get install systemd-container -y')

    def __upgr_sysd(self, cmd):
        try:
            raw = self._sys_call('systemd --version')

            ver = raw.split('\n')[0].split(' ')[1]
            logging.debug('systemd --version: {}'.format(ver))

            if int(ver) < 229:
                logging.info('Upgrade systemd')

                raw = self._sys_call(cmd)

                if self.recursion < 1:
                    self.recursion += 1

                    logging.info('Install systemd')
                    logging.debug(self.__upgr_sysd(cmd))
                else:
                    raise BaseException(raw)
                logging.info('Upgrade systemd done')

            logging.info('Systemd version: {}'.format(ver))
            self.recursion = 0

        except BaseException as sysexp:
            logging.error('Get/upgrade systemd ver: {}'.format(sysexp))
            sys.exit(1)

    def _finalizer(self, rw=None):
        f_path = main_conf['mark_final']
        if not isfile(f_path):
            self._file_rw(p=f_path, w=True, log='First start')
            return True
        else:
            if rw:
                self._file_rw(p=f_path, w=True, data='1')
                return True

            mark = self._file_rw(p=f_path)
            logging.debug('Start marker: {}'.format(mark))
            if not mark:
                logging.info('First start')
                return True

            logging.info('Second start.'
                         'This is protection against restarting the program.'
                         'If you need to re-run the script, '
                         'you need to delete the file {}'.format(f_path))
            return False

    def _byteify(self, data, ignore_dicts=False):
        if isinstance(data, unicode):
            return data.encode('utf-8')
        if isinstance(data, list):
            return [self._byteify(item, ignore_dicts=True) for item in data]
        if isinstance(data, dict) and not ignore_dicts:
            return {
                self._byteify(key, ignore_dicts=True): self._byteify(value,
                                                                     ignore_dicts=True)
                for key, value in data.iteritems()
            }
        return data

    def __json_load_byteified(self, file_handle):
        return self._byteify(
            load(file_handle, object_hook=self._byteify),
            ignore_dicts=True
        )

    def __get_url(self, link):
        resp = urlopen(url=link)
        return self.__json_load_byteified(resp)

    def build_cmd(self):
        conf = main_conf['build']

        json_db = self.__get_url(conf['conf_link'])
        db_conf = json_db.get('DB')
        if db_conf:
            conf['db_conf'].update(db_conf['Conn'])


        templ = str(self.__get_url(conf['templ'])).replace('\'', '"')
        conf['db_conf'] = str(conf['db_conf']).replace('\'', '"')

        conf['cmd'] = conf['cmd'].format(templ, conf['dappvpnconf_path'],
                                         conf['db_conf'])
        self._file_rw(p=conf['cmd_path'], w=True, data=conf['cmd'],
                      log='Create file with dapp cmd')


class Params(CMD):
    """ This class provide check sysctl and iptables """

    def __init__(self):
        self.f_vpn = main_conf['iptables']['unit_vpn']
        self.f_com = main_conf['iptables']['unit_com']
        self.p_dest = main_conf['iptables']['path_unit']
        self.p_dwld = main_conf['iptables']['path_download']
        self.params = main_conf['iptables']['unit_field']

    def run_service(self, sysctl, comm=False):
        if comm:
            logging.info('Run common service')
            self._sys_call('systemctl daemon-reload', sysctl)
            sleep(2)
            self._sys_call('systemctl enable {}'.format(self.f_com), sysctl)
            sleep(2)
            self._sys_call('systemctl start {}'.format(self.f_com), sysctl)
        else:
            logging.info('Run vpn service')
            self._sys_call('systemctl start {}'.format(self.f_vpn), sysctl)
            sleep(2)
            self._sys_call('systemctl enable {}'.format(self.f_vpn), sysctl)

    def __iptables(self):
        logging.debug('Check iptables')

        cmd = '/sbin/iptables -t nat -L'
        chain = 'Chain POSTROUTING'
        raw = self._sys_call(cmd)
        arr = raw.split('\n\n')
        chain_arr = []
        for i in arr:
            if chain in i:
                chain_arr = i.split('\n')
                break
        del arr

        addr = self.addres(chain_arr)
        infs = self.interfase()
        logging.debug('Addr,interface: {}'.format((addr, infs)))
        return addr, infs

    def interfase(self):
        def check_interfs(i):
            logging.info('Please enter one of your '
                         'available interfaces: {}'.format(i))

            new_intrfs = raw_input('>')
            if new_intrfs not in i:
                logging.info('Wrong. Interface must be one of: {}'.format(i))
                new_intrfs = check_interfs(i)
            return new_intrfs

        arr_intrfs = []
        cmd = 'LANG=POSIX lshw -C network'
        raw = self._sys_call(cmd)
        arr = raw.split('logical name: ')
        arr.pop(0)
        for i in arr:
            arr_intrfs.append(i.split('\n')[0])
        del arr
        if len(arr_intrfs) > 1:
            intrfs = check_interfs(arr_intrfs)
        else:
            intrfs = arr_intrfs[0]

        return intrfs

    def addres(self, arr):
        def check_addr(p):
            while True:
                addr = raw_input('>')
                match = search(p, addr)
                if not match:
                    logging.info('You addres is wrong,please enter '
                                 'right address.Example: 255.255.255.255')
                    addr = check_addr(p)
                break
            return addr

        addr = main_conf['addr'] + main_conf['mask'][0]
        for i in arr:
            if addr in i:
                logging.info('Addres {} is busy,'
                             'please enter free address.'.format(addr))

                pattern = r'^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$'
                # pattern = r'^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\/(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$'
                addr = check_addr(pattern)
                break
        return addr

    def __sysctl(self):
        """ Return True if ip_forward=1 by default,
        and False if installed by script """
        cmd = '/sbin/sysctl net.ipv4.ip_forward'
        res = self._sys_call(cmd).decode()
        param = int(res.split(' = ')[1])

        if not param:
            if self.recursion < 1:
                logging.debug('Change net.ipv4.ip_forward from 0 to 1')

                cmd = '/sbin/sysctl -w net.ipv4.ip_forward=1'
                self._sys_call(cmd)
                sleep(0.5)
                self.recursion += 1
                self.__sysctl()
                return False
            else:
                logging.error('sysctl net.ipv4.ip_forward didnt change to 1')
                sys.exit(3)
        return True

    def _rw_unit_file(self, ip, intfs, sysctl, code):
        logging.debug('Preparation unit file')
        addr = ip + main_conf['mask'][0]
        try:
            # read a list of lines into data
            tmp_data = self._file_rw(p=self.p_dwld + self.f_vpn)
            logging.debug('Read {}'.format(self.f_vpn))
            # replace all search fields
            for row in tmp_data:

                for param in self.params.keys():
                    if param in row:
                        indx = tmp_data.index(row)

                        if self.params[param]:
                            tmp_data[indx] = self.params[param].format(addr,
                                                                       intfs)
                        else:
                            if sysctl:
                                tmp_data[indx] = ''

            # rewrite unit file
            logging.debug('Rewrite {}'.format(self.f_vpn))
            self._file_rw(p=self.p_dwld + self.f_vpn, w=True, data=tmp_data)
            del tmp_data

            # move unit files
            logging.debug('Move units.')
            copyfile(self.p_dwld + self.f_vpn, self.p_dest + self.f_vpn)
            copyfile(self.p_dwld + self.f_com, self.p_dest + self.f_com)
        except BaseException as f_rw:
            logging.error('R/W unit file: {}'.format(f_rw))
            self._rolback(sysctl, code)

    def revise_params(self):
        sysctl = self.__sysctl()
        ip, intfs = self.__iptables()
        return ip, intfs, sysctl

    def _rw_openvpn_conf(self, new_ip, sysctl, code):
        # rewrite in /var/lib/container/vpn/etc/openvpn/config/server.conf
        # two fields: server,push "route",  if ip =! default addr.
        conf_file = "{}{}{}".format(main_conf['iptables']['path_download'],
                                    main_conf['iptables']['path_vpn'],
                                    main_conf['iptables']['openvpn_conf'])
        def_ip = main_conf['addr']
        def_mask = main_conf['mask'][1]
        search_fields = main_conf['iptables']['openvpn_fields']

        try:
            # read a list of lines into data
            tmp_data = self._file_rw(
                p=conf_file,
                log='Read openvpn server.conf'
            )

            # replace all search fields
            for row in tmp_data:

                for field in [f for f in search_fields]:
                    if field.format(def_ip, def_mask) in row:
                        indx = tmp_data.index(row)
                        tmp_data[indx] = field.format(new_ip, def_mask)

            # rewrite server.conf file
            self._file_rw(
                p=conf_file,
                w=True,
                data=tmp_data,
                log='Rewrite server.conf'
            )

            del tmp_data

            logging.debug('server.conf done')
        except BaseException as f_rw:
            logging.error('R/W server.conf: {}'.format(f_rw))
            self._rolback(sysctl, code)

    def _check_db_run(self, sysctl, code):
        # wait 't_wait' sec until the DB starts, if not started, exit.
        raw = self._file_rw(p=main_conf['build']['db_log'],
                            log='Read DB log')
        t_start = time()
        t_wait = 600
        while True:
            for i in raw:
                if main_conf['build']['db_stat'] in i:
                    logging.info('DB was run.')
                    break
            if time() - t_start > t_wait:
                logging.error(
                    'DB after {} sec does not run.'.format(t_wait))
                self._rolback(sysctl, code)
            sleep(1)

    def _run_dapp_cmd(self, sysctl):
        cmd = self._file_rw(p=main_conf['build']['cmd_path'],
                            log='Read dapp cmd')
        if cmd:
            self._sys_call(cmd=cmd[0], sysctl=sysctl)
            sleep(1)
        else:
            self._rolback(sysctl, 10)


class Rdata(CMD):
    def __init__(self):
        self.url = main_conf['iptables']['link_download']
        self.files = main_conf['iptables']['file_download']
        self.p_dwld = main_conf['iptables']['path_download']
        self.p_dest_vpn = main_conf['iptables']['path_vpn']
        self.p_dest_com = main_conf['iptables']['path_com']
        self.p_unpck = dict(vpn=self.p_dest_vpn, common=self.p_dest_com)

    def download(self, sysctl, code):
        try:
            logging.info('Begin download files.')

            if not isdir(self.p_dwld):
                mkdir(self.p_dwld)

            obj = URLopener()
            for f in self.files:
                logging.info('Start download {}.'.format(f))
                obj.retrieve(self.url + f, self.p_dwld + f)
                logging.info('Download {} done.'.format(f))
            return True

        except BaseException as down:
            logging.error('Download {}.'.format(down))
            self._rolback(sysctl, code)

    def unpacking(self, sysctl):
        logging.info('Begin unpacking download files.')
        try:
            for f in self.files:
                if '.tar.xz' == f[-7:]:
                    logging.info('Unpacking {}.'.format(f))
                    for k, v in self.p_unpck.items():
                        if k in f:
                            if not isdir(self.p_dwld + v):
                                mkdir(self.p_dwld + v)
                            cmd = 'tar xpf {} -C {} --numeric-owner'.format(
                                self.p_dwld + f, self.p_dwld + v)
                            self._sys_call(cmd, sysctl)
                            logging.info('Unpacking {} done.'.format(f))
        except BaseException as p_unpck:
            logging.error('Unpack: {}.'.format(p_unpck))

    def clean(self):
        logging.info('Delete downloaded files.')

        for f in self.files:
            logging.info('Delete {}'.format(f))
            remove(self.p_dwld + f)


class Checker(Params, Rdata):

    def __init__(self):
        Rdata.__init__(self)
        Params.__init__(self)
        self.task = dict(ubuntu=self._upgr_ub_pack,
                         debian=self._upgr_deb_pack
                         )

    def init_os(self):
        if self._finalizer():
            dist_name, ver, name_ver = linux_distribution()
            upgr_pack = self.task.get(dist_name.lower(), False)
            if not upgr_pack:
                logging.error('You system is {}.'
                              'She is not supported yet'.format(dist_name))
            upgr_pack(ver)
            ip, intfs, sysctl = self.revise_params()
            self.download(sysctl, 6)
            self.unpacking(sysctl)
            if not ip == main_conf['addr']:
                self._rw_openvpn_conf(ip, sysctl, 7)
            self._rw_unit_file(ip, intfs, sysctl, 5)
            self.clean()
            self.run_service(sysctl, comm=True)
            self._check_db_run(sysctl, 9)
            self._run_dapp_cmd(sysctl)
            self.run_service(sysctl)
            self._finalizer(True)


if __name__ == '__main__':
    args = sys.argv[1:]
    if args and args[0] == 'build':
        logging.info('Build mode.')
        CMD().build_cmd()
    elif not args:
        logging.info('Begin init.')
        Checker().init_os()
        logging.info('All done.')
    else:
        logging.error('Argument {} not allowed.'.format(args))