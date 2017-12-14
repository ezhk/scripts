#!/usr/bin/python3


import yaml
import sys
import modules.API.Yandex
import modules.web


URL_GET_EXTERNAL_IP = 'http://ipecho.net/plain'
CONFIG_PATH = 'update_ip.yml'


if __name__ == "__main__":
    config = None
    message = None

    (status, ip_addr) = modules.web.get_url_body(URL_GET_EXTERNAL_IP)
    if not status:
        print('error: cannot define local IPaddr')
        sys.exit(1)

    try:
        with open(CONFIG_PATH, 'r') as fh:
            config = yaml.load(fh)
    except BaseException as err:
            print(err)
            sys.exit(1)

    record_type = 'AAAA' if ':' in ip_addr else 'A'
    dns_obj = modules.API.Yandex.PDD_DNS(config['domain'],
                                         config['token'])

    (status, domains_desc) = dns_obj.list_domain()
    if not status or 'records' not in domains_desc:
        print('error: cannot get domain description')
        sys.exit(1)

    record_id = None
    for record_desc in domains_desc['records']:
        if 'subdomain' in record_desc and \
           record_desc['subdomain'] == config['subdomain']:
            if record_desc['content'] == ip_addr:
                sys.exit(0)

            record_id = record_desc['record_id']
            break

    if record_id:
        (status, message) = dns_obj.edit_domain({'record_id': record_id,
                                                 'type': record_type,
                                                 'subdomain': config['subdomain'],
                                                 'content': ip_addr,
                                                 'ttl': config['ttl']})
    else:
        (status, message) = dns_obj.add_domain({'type': record_type,
                                                'subdomain': config['subdomain'],
                                                'content': ip_addr,
                                                'ttl': config['ttl']})

    if not status:
        print('error: %s' % message)
        sys.exit(1)
